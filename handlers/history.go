package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bitcav/nitr-core/cpu"
	"github.com/bitcav/nitr-core/disk"
	"github.com/bitcav/nitr-core/ram"
	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/gofiber/fiber/v2"
)

// historySamplerDone mirrors bandwidthSamplerDone: closed by tests to stop a
// sampler goroutine started via StartHistorySampler so its shortened config
// and stubbed info funcs don't keep ticking into later tests. Production
// never stops the sampler; it runs for the life of the process.
var historySamplerDone chan struct{}

// StartHistorySampler launches the goroutine that retains CPU/RAM/disk/
// bandwidth samples in nitr.db, following StartBandwidthSampler's shape: one
// ticker, an immediate seed sample, and a done channel tests close. main
// only calls it when history_enabled is set (default off) — the sampler
// writes to disk every interval, a real behavior change on flash/SD-backed
// hosts (Raspberry Pi is a target deployment), so retention is opt-in.
func StartHistorySampler() {
	done := make(chan struct{})
	historySamplerDone = done
	interval := utils.HistorySampleInterval()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		sampleHistory()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				sampleHistory()
			}
		}
	}()
}

// cpuInfoFunc, ramInfoFunc and diskInfoFunc are seams so tests can stub the
// host reads without paying cpu.Info()'s ~1s of delta sleeps or depending on
// the host's actual disks. They point at the real functions in production.
var (
	cpuInfoFunc  = cpu.Info
	ramInfoFunc  = ram.Info
	diskInfoFunc = disk.Info
)

// sampleHistory gathers one sample per metric and persists it as a single
// batch, pruning anything older than the configured retention in the same
// transaction. Errors are logged, not raised: the sampler is a background
// loop with no caller to return an error to, and one bad tick must not kill
// retention for the life of the process.
func sampleHistory() {
	samples := make(map[string][]byte, 4)
	add := func(metric string, v any) {
		payload, err := json.Marshal(v)
		if err != nil {
			log.Printf("history: marshaling %s sample: %v", metric, err)
			return
		}
		samples[metric] = payload
	}
	add(db.MetricCPU, cpuInfoFunc())
	add(db.MetricRAM, ramInfoFunc())
	add(db.MetricDisks, diskInfoFunc())

	// Bandwidth comes from the cache StartBandwidthSampler keeps warm rather
	// than a fresh bandwidth.Info() — that would pay its hardcoded 1s delta
	// sleep a second time per tick, and the cache is at most a few seconds
	// stale. A nil cache (the first moments of process life, before the first
	// bandwidth sample lands) is skipped rather than stored as a null point.
	bandwidthCacheMu.RLock()
	bw := bandwidthCache
	bandwidthCacheMu.RUnlock()
	if bw != nil {
		add(db.MetricBandwidth, bw)
	}

	now := time.Now()
	cutoff := now.Add(-utils.HistoryRetention())
	if err := db.PutHistoryBatch(now, samples, cutoff); err != nil {
		log.Printf("history: retaining samples: %v", err)
	}
}

// historyRequested reports whether the caller asked for a time-range series:
// any of from/to/resolution switches the endpoint from its instantaneous
// form to the series form.
func historyRequested(c *fiber.Ctx) bool {
	return c.Query("from") != "" || c.Query("to") != "" || c.Query("resolution") != ""
}

// serveHistory answers a range query against the retained samples for
// metric. Defaults: from = the oldest retained sample, to = now, resolution
// = every stored sample. Retention disabled or unparsable parameters are
// 400s with the standard error envelope.
func serveHistory(c *fiber.Ctx, metric string) error {
	if !utils.HistoryEnabled() {
		return historyError(c, "metric history retention is disabled; set history_enabled (off by default) to use from/to/resolution")
	}

	from, err := parseTimeParam(c.Query("from"))
	if err != nil {
		return historyError(c, fmt.Sprintf("invalid from: %v", err))
	}
	to, err := parseTimeParam(c.Query("to"))
	if err != nil {
		return historyError(c, fmt.Sprintf("invalid to: %v", err))
	}
	resolution, err := parseResolution(c.Query("resolution"))
	if err != nil {
		return historyError(c, fmt.Sprintf("invalid resolution: %v", err))
	}

	samples, err := db.QueryHistory(metric, from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
			"status":  fiber.StatusInternalServerError,
		})
	}
	return c.JSON(thinSamples(samples, resolution))
}

func historyError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"message": message,
		"status":  fiber.StatusBadRequest,
	})
}

// parseTimeParam accepts RFC3339 ("2026-07-29T03:00:00Z") or bare Unix
// seconds ("1785316800"); empty means "no bound".
func parseTimeParam(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(sec, 0), nil
	}
	return time.Time{}, fmt.Errorf("%q is neither RFC3339 nor Unix seconds", s)
}

// parseResolution parses the resolution parameter as a non-negative number
// of seconds; empty or 0 means "return every stored sample".
func parseResolution(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil || sec < 0 {
		return 0, fmt.Errorf("%q is not a non-negative number of seconds", s)
	}
	return time.Duration(sec) * time.Second, nil
}

// thinSamples reduces a series to at most one sample per resolution window,
// keeping the first sample in each window. Thinning rather than averaging
// keeps every data payload byte-identical to what was sampled — averaging
// structured payloads (per-mount disks, per-interface bandwidth) would need
// per-metric aggregation logic for no accuracy gain at chart scale.
func thinSamples(samples []models.HistorySample, resolution time.Duration) []models.HistorySample {
	if resolution <= 0 || len(samples) < 2 {
		return samples
	}
	out := make([]models.HistorySample, 1, len(samples))
	out[0] = samples[0]
	last := samples[0].Timestamp
	for _, s := range samples[1:] {
		if s.Timestamp.Sub(last) >= resolution {
			out = append(out, s)
			last = s.Timestamp
		}
	}
	return out
}
