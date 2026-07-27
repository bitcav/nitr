package handlers

import (
	"fmt"
	"io/fs"
	"net/http/httptest"
	"testing"

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
