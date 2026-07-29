package handlers

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitcav/go-memdev"
	"github.com/bitcav/nitr-core/bandwidth"
	"github.com/bitcav/nitr-core/baseboard"
	"github.com/bitcav/nitr-core/bios"
	"github.com/bitcav/nitr-core/chassis"
	"github.com/bitcav/nitr-core/cpu"
	"github.com/bitcav/nitr-core/devices"
	"github.com/bitcav/nitr-core/disk"
	"github.com/bitcav/nitr-core/drive"
	"github.com/bitcav/nitr-core/gpu"
	"github.com/bitcav/nitr-core/host"
	"github.com/bitcav/nitr-core/isp"
	"github.com/bitcav/nitr-core/network"
	"github.com/bitcav/nitr-core/overview"
	"github.com/bitcav/nitr-core/product"
	"github.com/bitcav/nitr-core/ram"
	db "github.com/bitcav/nitr/database"
	"github.com/gofiber/fiber/v2"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gopsprocess "github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

func AuthAPI(c *fiber.Ctx) error {
	key := c.Get("x-api-key")
	storedKey, err := db.GetApiKey()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	if storedKey == key {
		return c.Next()
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"message": "Unauthorized, please correct the api key of the target host",
		"status":  fiber.StatusUnauthorized,
	})
}

// bandwidthInfoFunc is a seam so tests can stub bandwidth.Info() without
// paying its hardcoded 1s sleep. It points at bandwidth.Info in production.
var bandwidthInfoFunc = bandwidth.Info

// bandwidthSampleInterval controls how often the background sampler started
// by StartBandwidthSampler refreshes bandwidthCache. A var, not a const, so
// tests can shrink it instead of waiting on the production cadence.
var bandwidthSampleInterval = 5 * time.Second

var (
	bandwidthCacheMu sync.RWMutex
	bandwidthCache   []bandwidth.NetworkDeviceBandwidth
)

// bandwidthSamplerDone is closed by tests to stop a sampler goroutine
// started via StartBandwidthSampler, so a test's stubbed bandwidthInfoFunc
// and shortened bandwidthSampleInterval don't keep ticking into later tests
// once the test that started it returns. Production never stops the
// sampler; it runs for the life of the process.
var bandwidthSamplerDone chan struct{}

// StartBandwidthSampler launches a goroutine that keeps bandwidthCache fresh
// by calling bandwidthInfoFunc on bandwidthSampleInterval. bandwidth.Info()
// blocks for ~1s to compute its rx/tx delta; running it here, off the
// request path, is what lets Bandwidth answer immediately from cache
// instead of stalling every request a full second. Call once at startup.
func StartBandwidthSampler() {
	done := make(chan struct{})
	bandwidthSamplerDone = done
	go func() {
		ticker := time.NewTicker(bandwidthSampleInterval)
		defer ticker.Stop()
		sampleBandwidth()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				sampleBandwidth()
			}
		}
	}()
}

func sampleBandwidth() {
	result := bandwidthInfoFunc()
	bandwidthCacheMu.Lock()
	bandwidthCache = result
	bandwidthCacheMu.Unlock()
}

// Bandwidth returns a JSON response of the Bandwidth information, served
// from the cache StartBandwidthSampler keeps warm. Before the first sample
// completes (briefly, at startup) this returns an empty result rather than
// blocking the request on bandwidth.Info()'s 1s delta sample.
//
// With any of ?from=&to=&resolution= present it instead returns a series of
// retained samples (see history.go); without them the response is unchanged
// from before retention existed.
func Bandwidth(c *fiber.Ctx) error {
	if historyRequested(c) {
		return serveHistory(c, db.MetricBandwidth)
	}
	bandwidthCacheMu.RLock()
	defer bandwidthCacheMu.RUnlock()
	return c.JSON(bandwidthCache)
}

// Baseboard returns a JSON response of the Baseboard information
func Baseboard(c *fiber.Ctx) error {
	return c.JSON(baseboard.Info())
}

// Bios returns a JSON response of the Bios information
func Bios(c *fiber.Ctx) error {
	return c.JSON(bios.Info())
}

// Chassis returns a JSON response of the Chassis information
func Chassis(c *fiber.Ctx) error {
	return c.JSON(chassis.Info())
}

// CPU returns a JSON response of the CPUs information. With any of
// ?from=&to=&resolution= present it instead returns a series of retained
// samples (see history.go); without them the response is unchanged from
// before retention existed.
func CPU(c *fiber.Ctx) error {
	if historyRequested(c) {
		return serveHistory(c, db.MetricCPU)
	}
	return c.JSON(cpu.Info())
}

// Devices returns a JSON response of the Devices information
func Devices(c *fiber.Ctx) error {
	return c.JSON(devices.Info())
}

// Disk returns a JSON response of the Disks information. With any of
// ?from=&to=&resolution= present it instead returns a series of retained
// samples (see history.go); without them the response is unchanged from
// before retention existed.
func Disk(c *fiber.Ctx) error {
	if historyRequested(c) {
		return serveHistory(c, db.MetricDisks)
	}
	return c.JSON(disk.Info())
}

// Drive returns a JSON response of the Drives information
func Drive(c *fiber.Ctx) error {
	return c.JSON(drive.Info())
}

// GPU returns a JSON response of the GPUs information
func GPU(c *fiber.Ctx) error {
	return c.JSON(gpu.Info())
}

// Host returns a JSON response of the Host information
func Host(c *fiber.Ctx) error {
	return c.JSON(host.Info())
}

// ispInfoFunc is a seam so tests can stub isp.Info() without making its
// outbound, timeout-less HTTP call to speedtest.net. It points at
// isp.Info in production.
var ispInfoFunc = isp.Info

// ispCacheTTL is how long a successful ISP lookup is served from cache
// before ISP looks it up again. ISP/public IP changes rarely -- hours, not
// seconds -- so caching keeps isp.Info()'s outbound HTTP call off the
// common request path. A var, not a const, so tests can shrink it.
var ispCacheTTL = time.Hour

// ispTimeout bounds how long a single request will wait on isp.Info()
// before falling back to the cached (or empty) value instead of hanging.
// isp.Info() makes an outbound HTTP call with no timeout of its own and no
// context parameter to give it one, so the call runs on its own goroutine
// here and is raced against this deadline; a request never waits past it,
// though the goroutine underneath may still be blocked on the outbound call
// after ISP returns. A var, not a const, so tests can shrink it.
var ispTimeout = 5 * time.Second

var (
	ispCacheMu sync.Mutex
	ispCache   isp.Setting
	ispCacheAt time.Time
)

// ISP returns a JSON response of the ISP information, serving a cached
// result when it is fresh and degrading to the last good value (or empty,
// on a cold cache) when isp.Info() does not return within ispTimeout.
func ISP(c *fiber.Ctx) error {
	ispCacheMu.Lock()
	cached := ispCache
	fresh := !ispCacheAt.IsZero() && time.Since(ispCacheAt) < ispCacheTTL
	ispCacheMu.Unlock()
	if fresh {
		return c.JSON(cached)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ispTimeout)
	defer cancel()

	// Read the seam synchronously: capturing it before the goroutine keeps
	// the read on this request's goroutine rather than one that may still
	// be running (blocked in the outbound call) after ISP has already
	// returned via the timeout branch below.
	fn := ispInfoFunc
	result := make(chan isp.Setting, 1)
	go func() { result <- fn() }()

	select {
	case setting := <-result:
		ispCacheMu.Lock()
		ispCache = setting
		ispCacheAt = time.Now()
		ispCacheMu.Unlock()
		return c.JSON(setting)
	case <-ctx.Done():
		return c.JSON(cached)
	}
}

// Network returns a JSON response of the Network information
func Network(c *fiber.Ctx) error {
	return c.JSON(network.Info())
}

// Overview returns a JSON response of the Overview information
func Overview(c *fiber.Ctx) error {
	return c.JSON(overview.Info())
}

// ProcessInfo describes a single running process. CPUPercent is averaged
// over the process's lifetime (gopsutil's zero-interval mode), not a live
// instantaneous sample -- computing an instantaneous per-process rate would
// require sleeping once per process, which is the same class of blocking
// bug fixed in bandwidth.Info().
type ProcessInfo struct {
	Pid        int32   `json:"pid"`
	Ppid       int32   `json:"ppid"`
	Name       string  `json:"name"`
	User       string  `json:"user,omitempty"`
	Cmdline    string  `json:"cmdline,omitempty"`
	Status     string  `json:"status,omitempty"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float32 `json:"mem_percent"`
	RSS        uint64  `json:"rss"`
	StartTime  int64   `json:"start_time"`
}

// processesFunc is a seam so tests can stub the process list without
// depending on the host's actual running processes.
var processesFunc = defaultProcesses

// defaultProcesses lists every running process with as much detail as
// gopsutil can provide. A process that exits mid-scan (NewProcess/field
// lookups returning an error) is skipped rather than failing the whole
// call -- process listings are inherently racy against process exit, and
// one vanished PID should not blank out every other one.
func defaultProcesses() ([]ProcessInfo, error) {
	procs, err := gopsprocess.Processes()
	if err != nil {
		return nil, err
	}

	infos := make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}
		ppid, _ := p.Ppid()
		user, _ := p.Username()
		cmdline, _ := p.Cmdline()
		// gopsutil v4 returns the status as a slice; every supported
		// platform wraps the single v2-era string in one element.
		status := ""
		if st, _ := p.Status(); len(st) > 0 {
			status = st[0]
		}
		startTime, _ := p.CreateTime()
		cpuPercent, _ := p.CPUPercent()
		memPercent, _ := p.MemoryPercent()
		var rss uint64
		if mem, err := p.MemoryInfo(); err == nil && mem != nil {
			rss = mem.RSS
		}

		infos = append(infos, ProcessInfo{
			Pid:        p.Pid,
			Ppid:       ppid,
			Name:       name,
			User:       user,
			Cmdline:    cmdline,
			Status:     status,
			CPUPercent: cpuPercent,
			MemPercent: memPercent,
			RSS:        rss,
			StartTime:  startTime,
		})
	}
	return infos, nil
}

// Processes returns the process list in the same order GET /api/v1/processes
// returns with no query params: sorted by PID ascending (the switch's
// default case in Process below). Exported so the CLI info commands
// (`nitr processes`) can share this package's process-listing logic --
// including its gopsutil backend and panic-free error handling -- instead
// of duplicating it, and so `nitr processes --json` matches the bare API
// endpoint exactly rather than an independently-sorted copy.
func Processes() ([]ProcessInfo, error) {
	infos, err := processesFunc()
	if err != nil {
		return nil, err
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Pid < infos[j].Pid })
	return infos, nil
}

// Process returns a JSON response of the Processes information. Supports
// ?sort=cpu|mem|name|pid (default pid), ?order=asc|desc (default asc),
// ?limit=<n> and ?search=<substring>, matched case-insensitively against
// name and cmdline.
func Process(c *fiber.Ctx) error {
	infos, err := processesFunc()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
			"status":  fiber.StatusInternalServerError,
		})
	}

	if search := strings.ToLower(strings.TrimSpace(c.Query("search"))); search != "" {
		filtered := infos[:0]
		for _, p := range infos {
			if strings.Contains(strings.ToLower(p.Name), search) || strings.Contains(strings.ToLower(p.Cmdline), search) {
				filtered = append(filtered, p)
			}
		}
		infos = filtered
	}

	desc := strings.EqualFold(c.Query("order"), "desc")
	switch strings.ToLower(c.Query("sort")) {
	case "cpu":
		sort.Slice(infos, func(i, j int) bool { return less(infos[i].CPUPercent, infos[j].CPUPercent, desc) })
	case "mem":
		sort.Slice(infos, func(i, j int) bool { return less(infos[i].MemPercent, infos[j].MemPercent, desc) })
	case "name":
		sort.Slice(infos, func(i, j int) bool {
			if desc {
				return infos[i].Name > infos[j].Name
			}
			return infos[i].Name < infos[j].Name
		})
	default:
		sort.Slice(infos, func(i, j int) bool { return less(infos[i].Pid, infos[j].Pid, desc) })
	}

	if limitParam := c.Query("limit"); limitParam != "" {
		if limit, err := strconv.Atoi(limitParam); err == nil && limit >= 0 && limit < len(infos) {
			infos = infos[:limit]
		}
	}

	return c.JSON(infos)
}

// less orders two ordered values ascending, or descending when desc is true.
func less[T int32 | float64 | float32](a, b T, desc bool) bool {
	if desc {
		return a > b
	}
	return a < b
}

// Product returns a JSON response of the Product information
func Product(c *fiber.Ctx) error {
	return c.JSON(product.Info())
}

// RAM returns a JSON response of the RAM information. With any of
// ?from=&to=&resolution= present it instead returns a series of retained
// samples (see history.go); without them the response is unchanged from
// before retention existed.
func RAM(c *fiber.Ctx) error {
	if historyRequested(c) {
		return serveHistory(c, db.MetricRAM)
	}
	return c.JSON(ram.Info())
}

// swapInfoFunc is a seam so tests can stub mem.SwapMemory() without
// depending on the host's actual swap configuration.
var swapInfoFunc = mem.SwapMemory

// Swap returns a JSON response of the swap memory information. /ram only
// reports physical memory, so a host swapping itself to death looks fine
// there -- this is the number that catches it.
func Swap(c *fiber.Ctx) error {
	swap, err := swapInfoFunc()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
			"status":  fiber.StatusInternalServerError,
		})
	}
	return c.JSON(swap)
}

// loadAvgFunc is a seam so tests can stub load.Avg() deterministically.
var loadAvgFunc = load.Avg

// LoadAvg returns a JSON response of the 1/5/15 minute load average -- the
// standard Unix health number. Not implemented on Windows (gopsutil has no
// equivalent concept there); that surfaces as 501 rather than a silently
// empty 200, so a caller can distinguish "unsupported platform" from
// "broken".
func LoadAvg(c *fiber.Ctx) error {
	avg, err := loadAvgFunc()
	if err != nil {
		code := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "not implemented") {
			code = fiber.StatusNotImplemented
		}
		return c.Status(code).JSON(fiber.Map{
			"message": err.Error(),
			"status":  code,
		})
	}
	return c.JSON(avg)
}

// sensorsInfoFunc is a seam so tests can stub host.SensorsTemperatures()
// without depending on the host's actual hardware sensors.
var sensorsInfoFunc = sensors.SensorsTemperatures

// Sensors returns a JSON response of the available temperature/fan sensor
// readings. The top request for any hardware monitor, especially on the
// Raspberry Pi / home-server hosts this tool suits. A host with no exposed
// sensors is not an error -- it returns whatever sensorsInfoFunc gives
// back (nil serialises to null), the same as every other list endpoint's
// behaviour on an empty result (gpu, devices, drives, network).
//
// gopsutil reads one sysfs file per sensor and aggregates any per-file
// failure into a non-nil "warnings" error while still returning every
// sensor that DID read successfully -- one unreadable hwmon entry (seen in
// practice: an ACPI power-supply hwmon node with no temp*_input file) must
// not blank out every other sensor on the host. Only an error with no data
// at all is a real failure.
func Sensors(c *fiber.Ctx) error {
	temps, err := sensorsInfoFunc()
	if err != nil && len(temps) == 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
			"status":  fiber.StatusInternalServerError,
		})
	}
	return c.JSON(temps)
}

// memdevInfoFunc is a seam so tests can stub memdev.Info() without needing
// SMBIOS access. It points at memdev.Info in production.
var memdevInfoFunc = memdev.Info

// Memory returns a JSON response of the Memory Devices
func Memory(c *fiber.Ctx) error {
	memInfo, err := memdevInfoFunc()
	if err != nil {
		code := fiber.StatusInternalServerError
		if errors.Is(err, fs.ErrPermission) {
			code = fiber.StatusForbidden
		}
		return c.Status(code).JSON(fiber.Map{
			"message": err.Error(),
			"status":  code,
		})
	}
	return c.JSON(memInfo)
}
