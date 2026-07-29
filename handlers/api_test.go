package handlers

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bitcav/go-memdev"
	"github.com/bitcav/nitr-core/bandwidth"
	"github.com/bitcav/nitr-core/isp"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthAPIAllowed(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/cpu", AuthAPI, func(c *fiber.Ctx) error { return c.SendString("granted") })

	req := httptest.NewRequest("GET", "/cpu", nil)
	req.Header.Set("x-api-key", "testapikey")
	resp, err := app.Test(req, 30000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "granted", body(t, resp))
}

func TestAuthAPIDeniedMissingKey(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/cpu", AuthAPI, func(c *fiber.Ctx) error { return c.SendString("granted") })

	resp := get(t, app, "/cpu")
	assert.Equal(t, 401, resp.StatusCode)
	bd := body(t, resp)
	assert.Contains(t, bd, "Unauthorized")
	assert.Contains(t, bd, "\"status\":401")
}

func TestAuthAPIDeniedWrongKey(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/cpu", AuthAPI, func(c *fiber.Ctx) error { return c.SendString("granted") })

	req := httptest.NewRequest("GET", "/cpu", nil)
	req.Header.Set("x-api-key", "wrong")
	resp, err := app.Test(req, 30000)
	assert.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
	assert.Contains(t, body(t, resp), "Unauthorized")
}

func TestAPIEndpoints(t *testing.T) {
	setupEnv(t)

	// bandwidth.Info() sleeps a hardcoded 1s and isp.Info() makes an
	// outbound, timeout-less HTTP call to speedtest.net. Stub both via
	// the package-level seams so the test stays local and fast.
	origBandwidth := bandwidthInfoFunc
	origISP := ispInfoFunc
	bandwidthInfoFunc = func() []bandwidth.NetworkDeviceBandwidth { return nil }
	ispInfoFunc = func() isp.Setting { return isp.Setting{} }
	t.Cleanup(func() {
		bandwidthInfoFunc = origBandwidth
		ispInfoFunc = origISP
	})

	type tc struct {
		path    string
		handler fiber.Handler
	}
	cases := []tc{
		{"/cpu", CPU},
		{"/bios", Bios},
		{"/baseboard", Baseboard},
		{"/chassis", Chassis},
		{"/devices", Devices},
		{"/disk", Disk},
		{"/drive", Drive},
		{"/gpu", GPU},
		{"/host", Host},
		{"/network", Network},
		{"/overview", Overview},
		{"/process", Process},
		{"/product", Product},
		{"/ram", RAM},
		{"/bandwidth", Bandwidth},
		{"/isp", ISP},
		{"/memory", Memory},
	}

	for _, c := range cases {
		c := c
		t.Run(c.path, func(t *testing.T) {
			app := newTestApp()
			app.Get(c.path, AuthAPI, c.handler)

			req := httptest.NewRequest("GET", c.path, nil)
			req.Header.Set("x-api-key", "testapikey")
			resp, err := app.Test(req, 30000)
			require.NoError(t, err, "app.Test timed out or failed; resp would be nil")
			// the handler must have executed (status not mutated -> 200,
			// 500 if a system call panicked and was recovered, or 403 if
			// /memory hit a permission error on a root-less host).
			assert.Contains(t, []int{200, 403, 500}, resp.StatusCode)
			// drain body so the connection can be released
			_ = body(t, resp)
		})
	}
}

// TestMemoryErrorHandling covers the previously-swallowed error path.
// Each case injects a concrete return value through the memdevInfoFunc seam
// so the test is deterministic and host-independent.
//
// Against the old code (fmt.Println(err); return c.JSON(memInfo)) every
// error case below would return 200 null instead of the asserted status,
// so these guards fail loudly if the swallow regression returns.
// TestBandwidthServesFromCache proves Bandwidth never calls bandwidthInfoFunc
// itself -- it only reads bandwidthCache -- so a request can never pay
// bandwidth.Info()'s 1s delta-sample cost. bandwidthInfoFunc is stubbed to
// panic if invoked: against the old code (which called it directly) this
// test would panic.
func TestBandwidthServesFromCache(t *testing.T) {
	origCache := bandwidthCache
	origFunc := bandwidthInfoFunc
	bandwidthInfoFunc = func() []bandwidth.NetworkDeviceBandwidth {
		t.Fatal("Bandwidth handler must not call bandwidthInfoFunc directly")
		return nil
	}
	bandwidthCache = []bandwidth.NetworkDeviceBandwidth{{Name: "eth0", RxBytes: 42}}
	t.Cleanup(func() {
		bandwidthCache = origCache
		bandwidthInfoFunc = origFunc
	})

	app := newTestApp()
	app.Get("/bandwidth", Bandwidth)

	resp := get(t, app, "/bandwidth")
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, body(t, resp), `"rxBytes":42`)
}

// TestStartBandwidthSamplerPopulatesCache verifies the background sampler
// actually calls bandwidthInfoFunc and writes the result into bandwidthCache,
// off the goroutine that started it. The sampler is stopped via
// bandwidthSamplerDone before the test returns so its shortened interval and
// stubbed bandwidthInfoFunc don't keep ticking into later tests.
func TestStartBandwidthSamplerPopulatesCache(t *testing.T) {
	origCache := bandwidthCache
	origFunc := bandwidthInfoFunc
	origInterval := bandwidthSampleInterval
	var calls int32
	want := []bandwidth.NetworkDeviceBandwidth{{Name: "eth0", RxBytes: 7}}
	bandwidthInfoFunc = func() []bandwidth.NetworkDeviceBandwidth {
		atomic.AddInt32(&calls, 1)
		return want
	}
	bandwidthCache = nil
	bandwidthSampleInterval = time.Hour // only the immediate seed sample should fire
	t.Cleanup(func() {
		close(bandwidthSamplerDone)
		bandwidthCache = origCache
		bandwidthInfoFunc = origFunc
		bandwidthSampleInterval = origInterval
	})

	StartBandwidthSampler()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&calls) >= 1
	}, time.Second, time.Millisecond, "sampler did not call bandwidthInfoFunc")

	bandwidthCacheMu.RLock()
	got := bandwidthCache
	bandwidthCacheMu.RUnlock()
	assert.Equal(t, want, got)
}

// TestISPCachesResult proves a second request within ispCacheTTL is served
// from cache instead of calling ispInfoFunc again -- the mechanism that
// keeps isp.Info()'s outbound HTTP call off the common request path.
func TestISPCachesResult(t *testing.T) {
	origCache, origAt, origFunc := ispCache, ispCacheAt, ispInfoFunc
	var calls int32
	want := isp.Setting{Isp: "Test ISP", IP: "1.2.3.4"}
	ispInfoFunc = func() isp.Setting {
		atomic.AddInt32(&calls, 1)
		return want
	}
	ispCache, ispCacheAt = isp.Setting{}, time.Time{}
	t.Cleanup(func() {
		ispCache, ispCacheAt, ispInfoFunc = origCache, origAt, origFunc
	})

	app := newTestApp()
	app.Get("/isp", ISP)

	resp := get(t, app, "/isp")
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, body(t, resp), "Test ISP")

	resp = get(t, app, "/isp")
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, body(t, resp), "Test ISP")

	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "second request within ispCacheTTL must be served from cache")
}

// TestISPTimeoutFallsBackToCache proves a hung ispInfoFunc cannot hang the
// request: against the old code (return c.JSON(ispInfoFunc())) this test
// would time out entirely rather than complete quickly with the stale value.
func TestISPTimeoutFallsBackToCache(t *testing.T) {
	origCache, origAt, origFunc := ispCache, ispCacheAt, ispInfoFunc
	origTimeout, origTTL := ispTimeout, ispCacheTTL
	block := make(chan struct{}) // never closed: simulates a hung isp.Info()
	ispInfoFunc = func() isp.Setting {
		<-block
		return isp.Setting{}
	}
	// A stale-but-present cached value: old enough that ispCacheTTL no
	// longer considers it fresh, so ISP attempts (and then must time out on)
	// a fresh lookup rather than short-circuiting to the cache.
	ispCache = isp.Setting{Isp: "Stale ISP"}
	ispCacheAt = time.Now().Add(-time.Hour)
	ispCacheTTL = time.Millisecond
	ispTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		ispCache, ispCacheAt, ispInfoFunc = origCache, origAt, origFunc
		ispTimeout, ispCacheTTL = origTimeout, origTTL
	})

	app := newTestApp()
	app.Get("/isp", ISP)

	start := time.Now()
	resp := get(t, app, "/isp")
	elapsed := time.Since(start)

	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, body(t, resp), "Stale ISP")
	assert.Less(t, elapsed, 500*time.Millisecond, "ISP must not block past ispTimeout")
}

func TestMemoryErrorHandling(t *testing.T) {
	setupEnv(t)

	type tc struct {
		name       string
		infoReturn func() ([]memdev.Memory, error)
		wantStatus int
		wantInBody []string
	}
	cases := []tc{
		{
			name:       "permission error maps to 403",
			infoReturn: func() ([]memdev.Memory, error) { return nil, fs.ErrPermission },
			wantStatus: 403,
			wantInBody: []string{`"status":403`, `"message"`},
		},
		{
			name:       "wrapped EACCES-style permission error maps to 403",
			infoReturn: func() ([]memdev.Memory, error) { return nil, fmt.Errorf("open /dev/mem: %w", fs.ErrPermission) },
			wantStatus: 403,
			wantInBody: []string{`"status":403`, `"message"`},
		},
		{
			name:       "non-permission error maps to 500",
			infoReturn: func() ([]memdev.Memory, error) { return nil, fmt.Errorf("SMBIOS table corrupt") },
			wantStatus: 500,
			wantInBody: []string{`"status":500`, `"message"`, "SMBIOS table corrupt"},
		},
		{
			name: "success returns 200 with body",
			infoReturn: func() ([]memdev.Memory, error) {
				return []memdev.Memory{{Bank: "DIMM0", Size: 8192, Unit: "MB"}}, nil
			},
			wantStatus: 200,
			wantInBody: []string{`"bank":"DIMM0"`},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			orig := memdevInfoFunc
			memdevInfoFunc = c.infoReturn
			t.Cleanup(func() { memdevInfoFunc = orig })

			app := newTestApp()
			app.Get("/memory", AuthAPI, Memory)

			req := httptest.NewRequest("GET", "/memory", nil)
			req.Header.Set("x-api-key", "testapikey")
			resp, err := app.Test(req, 30000)
			require.NoError(t, err)
			assert.Equal(t, c.wantStatus, resp.StatusCode)
			bd := body(t, resp)
			for _, s := range c.wantInBody {
				assert.Contains(t, bd, s)
			}
		})
	}
}

// TestProcessesMatchesDefaultHandlerResponseExactly is the proof behind the
// CLI info-commands ticket's acceptance criterion ("`nitr processes --json`
// matches `GET /api/v1/processes` exactly"): the CLI calls Processes()
// directly (no HTTP, no fiber) while the API goes through the Process fiber
// handler, so this pins that both paths apply the identical default sort
// (PID ascending) to the identical snapshot and therefore json.Marshal to
// the identical bytes, not just "look similar".
func TestProcessesMatchesDefaultHandlerResponseExactly(t *testing.T) {
	setupEnv(t)

	orig := processesFunc
	processesFunc = func() ([]ProcessInfo, error) {
		// Deliberately out of PID order, so a test that forgot to sort
		// would still fail.
		return []ProcessInfo{{Pid: 30}, {Pid: 10}, {Pid: 20}}, nil
	}
	t.Cleanup(func() { processesFunc = orig })

	app := newTestApp()
	app.Get("/processes", AuthAPI, Process)
	req := httptest.NewRequest("GET", "/processes", nil)
	req.Header.Set("x-api-key", "testapikey")
	resp, err := app.Test(req, 30000)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	apiBody := body(t, resp)

	infos, err := Processes()
	require.NoError(t, err)
	cliRaw, err := json.Marshal(infos)
	require.NoError(t, err)

	assert.Equal(t, apiBody, string(cliRaw))
	assert.Equal(t, `[{"pid":10,"ppid":0,"name":"","cpu_percent":0,"mem_percent":0,"rss":0,"start_time":0},`+
		`{"pid":20,"ppid":0,"name":"","cpu_percent":0,"mem_percent":0,"rss":0,"start_time":0},`+
		`{"pid":30,"ppid":0,"name":"","cpu_percent":0,"mem_percent":0,"rss":0,"start_time":0}]`, apiBody)
}

// TestProcessErrorHandling covers the panic-to-error conversion in
// process.Info() (nitr-core, unvendored). Against the old code
// (return c.JSON(process.Info())), a listing failure panicked the request
// instead of returning a mapped status -- there was no error path to test.
func TestProcessErrorHandling(t *testing.T) {
	setupEnv(t)

	orig := processesFunc
	processesFunc = func() ([]ProcessInfo, error) { return nil, fmt.Errorf("could not list processes") }
	t.Cleanup(func() { processesFunc = orig })

	app := newTestApp()
	app.Get("/processes", AuthAPI, Process)

	req := httptest.NewRequest("GET", "/processes", nil)
	req.Header.Set("x-api-key", "testapikey")
	resp, err := app.Test(req, 30000)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	bd := body(t, resp)
	assert.Contains(t, bd, `"status":500`)
	assert.Contains(t, bd, "could not list processes")
}

func stubProcesses() []ProcessInfo {
	return []ProcessInfo{
		{Pid: 3, Name: "beta", Cmdline: "/usr/bin/beta --flag", CPUPercent: 10, MemPercent: 5},
		{Pid: 1, Name: "alpha", Cmdline: "/usr/bin/alpha", CPUPercent: 30, MemPercent: 1},
		{Pid: 2, Name: "gamma", Cmdline: "/usr/bin/gamma", CPUPercent: 20, MemPercent: 15},
	}
}

func TestProcessSortOrderLimitSearch(t *testing.T) {
	setupEnv(t)

	orig := processesFunc
	t.Cleanup(func() { processesFunc = orig })

	type tc struct {
		name       string
		query      string
		wantPids   []string // in expected order, as they must appear in the JSON array
		wantStatus int
	}
	cases := []tc{
		{
			name:     "default sorts by pid ascending",
			query:    "",
			wantPids: []string{`"pid":1`, `"pid":2`, `"pid":3`},
		},
		{
			name:     "sort=pid&order=desc reverses default",
			query:    "?sort=pid&order=desc",
			wantPids: []string{`"pid":3`, `"pid":2`, `"pid":1`},
		},
		{
			name:     "sort=cpu&order=desc",
			query:    "?sort=cpu&order=desc",
			wantPids: []string{`"pid":1`, `"pid":2`, `"pid":3`},
		},
		{
			name:     "sort=mem ascending",
			query:    "?sort=mem",
			wantPids: []string{`"pid":1`, `"pid":3`, `"pid":2`},
		},
		{
			name:     "sort=name descending",
			query:    "?sort=name&order=desc",
			wantPids: []string{`"pid":2`, `"pid":3`, `"pid":1`}, // gamma, beta, alpha
		},
		{
			name:     "limit truncates after sort",
			query:    "?sort=cpu&order=desc&limit=2",
			wantPids: []string{`"pid":1`, `"pid":2`},
		},
		{
			name:     "search matches name case-insensitively",
			query:    "?search=ALPHA",
			wantPids: []string{`"pid":1`},
		},
		{
			name:     "search matches cmdline substring",
			query:    "?search=--flag",
			wantPids: []string{`"pid":3`},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			processesFunc = func() ([]ProcessInfo, error) { return stubProcesses(), nil }

			app := newTestApp()
			app.Get("/processes", AuthAPI, Process)

			req := httptest.NewRequest("GET", "/processes"+c.query, nil)
			req.Header.Set("x-api-key", "testapikey")
			resp, err := app.Test(req, 30000)
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			bd := body(t, resp)

			lastIdx := -1
			for _, want := range c.wantPids {
				idx := strings.Index(bd, want)
				assert.GreaterOrEqual(t, idx, 0, "expected %q in body %s", want, bd)
				assert.Greater(t, idx, lastIdx, "expected %q to appear after previous entry in %s", want, bd)
				lastIdx = idx
			}
			if c.name == "limit truncates after sort" {
				assert.Equal(t, 2, strings.Count(bd, `"pid":`))
			}
		})
	}
}
