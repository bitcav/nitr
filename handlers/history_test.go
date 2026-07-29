package handlers

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bitcav/nitr-core/bandwidth"
	"github.com/bitcav/nitr-core/cpu"
	"github.com/bitcav/nitr-core/disk"
	"github.com/bitcav/nitr-core/ram"
	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTimeParam(t *testing.T) {
	// empty = no bound
	got, err := parseTimeParam("")
	require.NoError(t, err)
	assert.True(t, got.IsZero())

	// RFC3339
	got, err = parseTimeParam("2026-07-29T03:00:00Z")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 7, 29, 3, 0, 0, 0, time.UTC), got.UTC())

	// Unix seconds
	got, err = parseTimeParam("1785316800")
	require.NoError(t, err)
	assert.Equal(t, int64(1785316800), got.Unix())

	// garbage is a parse error, not a silent default
	_, err = parseTimeParam("yesterday-ish")
	assert.Error(t, err)
}

func TestParseResolution(t *testing.T) {
	got, err := parseResolution("")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), got)

	got, err = parseResolution("60")
	require.NoError(t, err)
	assert.Equal(t, time.Minute, got)

	_, err = parseResolution("-5")
	assert.Error(t, err)
	_, err = parseResolution("fast")
	assert.Error(t, err)
}

func TestThinSamples(t *testing.T) {
	base := time.Unix(1785316800, 0)
	series := make([]models.HistorySample, 0, 5)
	for i := 0; i < 5; i++ { // 10s apart
		series = append(series, models.HistorySample{
			Timestamp: base.Add(time.Duration(i) * 10 * time.Second),
			Data:      json.RawMessage(fmt.Sprintf(`{"i":%d}`, i)),
		})
	}

	// resolution 0 keeps everything
	assert.Len(t, thinSamples(series, 0), 5)
	// 25s window keeps the first sample of each window: t0, t30 (t40-t30=10s
	// is inside the window) -> 2 samples, payloads untouched
	thinned := thinSamples(series, 25*time.Second)
	require.Len(t, thinned, 2)
	assert.Equal(t, `{"i":0}`, string(thinned[0].Data))
	assert.Equal(t, `{"i":3}`, string(thinned[1].Data))
	// degenerate inputs pass through
	assert.Len(t, thinSamples(series[:1], time.Minute), 1)
	assert.Empty(t, thinSamples(nil, time.Minute))
}

// historyTestSeeds stubs the host-read seams and viper config for history
// tests, returning cleanup responsibility to t.Cleanup.
func enableHistory(t *testing.T) {
	t.Helper()
	viper.Set("history_enabled", true)
	t.Cleanup(viper.Reset)
}

// TestHistoryRangeDisabledReturns400 is the default-config guarantee: with
// retention off, asking for a series is a clear 400 pointing at the config
// key, not a silently empty 200.
func TestHistoryRangeDisabledReturns400(t *testing.T) {
	setupEnv(t)
	t.Cleanup(viper.Reset) // ensure history_enabled stays unset even if a prior test leaked it

	app := newTestApp()
	app.Get("/cpu", CPU)

	resp := get(t, app, "/cpu?from=1785316800")
	assert.Equal(t, 400, resp.StatusCode)
	assert.Contains(t, body(t, resp), "history_enabled")
}

// TestHistoryRangeInvalidParamsReturns400 covers the parse-error envelope.
func TestHistoryRangeInvalidParamsReturns400(t *testing.T) {
	setupEnv(t)
	enableHistory(t)

	app := newTestApp()
	app.Get("/cpu", CPU)

	for _, target := range []string{
		"/cpu?from=not-a-time",
		"/cpu?to=not-a-time",
		"/cpu?resolution=-1",
	} {
		resp := get(t, app, target)
		assert.Equal(t, 400, resp.StatusCode, target)
		assert.Contains(t, body(t, resp), `"status":400`, target)
	}
}

// TestHistoryRangeReturnsSeries seeds retained samples directly and proves
// the endpoint returns them as a [{timestamp, data}] series with range
// filtering and resolution thinning applied.
func TestHistoryRangeReturnsSeries(t *testing.T) {
	setupEnv(t)
	enableHistory(t)

	base := time.Unix(1785316800, 0).UTC()
	for i := 0; i < 4; i++ { // 10s apart
		ts := base.Add(time.Duration(i) * 10 * time.Second)
		require.NoError(t, db.PutHistoryBatch(ts, map[string][]byte{
			db.MetricRAM: []byte(fmt.Sprintf(`{"total":8,"free":%d,"usage":%d}`, i, 8-i)),
		}, ts.Add(-time.Hour)))
	}

	app := newTestApp()
	app.Get("/ram", RAM)

	// full range
	resp := get(t, app, fmt.Sprintf("/ram?from=%d&to=%d", base.Unix(), base.Add(30*time.Second).Unix()))
	require.Equal(t, 200, resp.StatusCode)
	var series []models.HistorySample
	require.NoError(t, json.Unmarshal([]byte(body(t, resp)), &series))
	require.Len(t, series, 4)
	assert.Equal(t, `{"total":8,"free":0,"usage":8}`, string(series[0].Data))
	assert.True(t, series[0].Timestamp.Equal(base))

	// interior window, inclusive bounds, RFC3339 form
	resp = get(t, app, "/ram?from="+base.Add(10*time.Second).Format(time.RFC3339)+
		"&to="+base.Add(20*time.Second).Format(time.RFC3339))
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.Unmarshal([]byte(body(t, resp)), &series))
	require.Len(t, series, 2)
	assert.Equal(t, `{"total":8,"free":1,"usage":7}`, string(series[0].Data))

	// resolution thins to at most one sample per window
	resp = get(t, app, fmt.Sprintf("/ram?from=%d&to=%d&resolution=25", base.Unix(), base.Add(30*time.Second).Unix()))
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.Unmarshal([]byte(body(t, resp)), &series))
	assert.Len(t, series, 2)

	// a range with no samples is [], not null
	resp = get(t, app, fmt.Sprintf("/ram?from=%d&to=%d", base.Add(time.Hour).Unix(), base.Add(2*time.Hour).Unix()))
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "[]", body(t, resp))
}

// TestHistoryNoParamsKeepsInstantaneousShape is the backward-compatibility
// guard: with no range parameters the four history-capable endpoints must
// return exactly today's instantaneous shape (object/array, not a series).
func TestHistoryNoParamsKeepsInstantaneousShape(t *testing.T) {
	setupEnv(t)
	enableHistory(t) // even with retention ON, no params = old behavior

	origCache := bandwidthCache
	bandwidthCache = []bandwidth.NetworkDeviceBandwidth{{Name: "eth0", RxBytes: 42}}
	t.Cleanup(func() { bandwidthCache = origCache })

	app := newTestApp()
	app.Get("/ram", RAM)
	app.Get("/bandwidth", Bandwidth)

	// RAM: a bare object with total/free/usage, no timestamp wrapper
	resp := get(t, app, "/ram")
	require.Equal(t, 200, resp.StatusCode)
	var ramPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(body(t, resp)), &ramPayload))
	assert.Contains(t, ramPayload, "total")
	assert.NotContains(t, ramPayload, "timestamp")

	// Bandwidth: the bare per-interface array from the cache
	resp = get(t, app, "/bandwidth")
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, `[{"name":"eth0","rxBytes":42,"txBytes":0,"rxPackets":0,"txPackets":0}]`, body(t, resp))
}

// TestSampleHistoryPersistsAllMetrics drives one sampler tick with stubbed
// host reads and proves a queryable sample lands per metric, including
// bandwidth sourced from the shared cache.
func TestSampleHistoryPersistsAllMetrics(t *testing.T) {
	setupEnv(t)
	enableHistory(t)

	origCPU, origRAM, origDisk := cpuInfoFunc, ramInfoFunc, diskInfoFunc
	origCache := bandwidthCache
	cpuInfoFunc = func() cpu.CPU { return cpu.CPU{Usage: 55} }
	ramInfoFunc = func() ram.RAM { return ram.RAM{Total: 8, Free: 4, Usage: 4} }
	diskInfoFunc = func() []disk.Disk { return []disk.Disk{{Mountpoint: "/", Percent: 50}} }
	bandwidthCache = []bandwidth.NetworkDeviceBandwidth{{Name: "eth0", RxBytes: 7}}
	t.Cleanup(func() {
		cpuInfoFunc, ramInfoFunc, diskInfoFunc = origCPU, origRAM, origDisk
		bandwidthCache = origCache
	})

	sampleHistory()

	for metric, want := range map[string]string{
		db.MetricCPU:       `"usage":55`,
		db.MetricRAM:       `"total":8`,
		db.MetricDisks:     `"percent":50`,
		db.MetricBandwidth: `"rxBytes":7`,
	} {
		got, err := db.QueryHistory(metric, time.Time{}, time.Time{})
		require.NoError(t, err)
		require.Len(t, got, 1, "metric %s", metric)
		assert.Contains(t, string(got[0].Data), want, "metric %s", metric)
	}
}

// TestStartHistorySamplerTicks proves the sampler goroutine actually runs on
// its interval and stops on historySamplerDone, mirroring
// TestStartBandwidthSamplerPopulatesCache.
func TestStartHistorySamplerTicks(t *testing.T) {
	setupEnv(t)
	enableHistory(t)
	viper.Set("history_interval", 1) // 1s floor: first tick is the seed sample

	origCPU, origRAM, origDisk := cpuInfoFunc, ramInfoFunc, diskInfoFunc
	origCache := bandwidthCache
	cpuInfoFunc = func() cpu.CPU { return cpu.CPU{Usage: 1} }
	ramInfoFunc = func() ram.RAM { return ram.RAM{Total: 1} }
	diskInfoFunc = func() []disk.Disk { return nil }
	bandwidthCache = nil // nil cache must be skipped, not stored as null
	t.Cleanup(func() {
		close(historySamplerDone)
		cpuInfoFunc, ramInfoFunc, diskInfoFunc = origCPU, origRAM, origDisk
		bandwidthCache = origCache
	})

	StartHistorySampler()

	require.Eventually(t, func() bool {
		got, err := db.QueryHistory(db.MetricCPU, time.Time{}, time.Time{})
		return err == nil && len(got) >= 1
	}, 2*time.Second, 10*time.Millisecond, "sampler did not retain a cpu sample")

	// nil bandwidth cache: no bandwidth bucket may appear
	got, err := db.QueryHistory(db.MetricBandwidth, time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

// TestHistoryRangeOnCPURoute seeds cpu samples and exercises the /cpu route
// end to end through httptest, confirming the route wiring (not just the
// shared serveHistory helper) reaches the right bucket.
func TestHistoryRangeOnCPURoute(t *testing.T) {
	setupEnv(t)
	enableHistory(t)

	ts := time.Unix(1785316800, 0).UTC()
	require.NoError(t, db.PutHistoryBatch(ts, map[string][]byte{
		db.MetricCPU: []byte(`{"vendor":"test","model":"m","cores":1,"threads":1,"clockSpeed":1000,"usage":42,"usageEach":[42]}`),
	}, ts.Add(-time.Hour)))

	app := newTestApp()
	app.Get("/cpu", CPU)

	req := httptest.NewRequest("GET", fmt.Sprintf("/cpu?from=%d&to=%d", ts.Unix(), ts.Unix()), nil)
	resp, err := app.Test(req, 30000)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	b := body(t, resp)
	assert.Contains(t, b, `"timestamp":"2026-07-29T`)
	assert.Contains(t, b, `"usage":42`)
}
