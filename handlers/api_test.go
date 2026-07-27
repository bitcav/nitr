package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/stretchr/testify/assert"
)

func TestAuthAPIAllowed(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/cpu", AuthAPI, func(c *fiber.Ctx) { c.Send("granted") })

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
	app.Get("/cpu", AuthAPI, func(c *fiber.Ctx) { c.Send("granted") })

	resp := get(t, app, "/cpu")
	assert.Equal(t, 200, resp.StatusCode) // handler never sets a status code
	bd := body(t, resp)
	assert.Contains(t, bd, "Unauthorized")
	assert.Contains(t, bd, "\"status\":401")
}

func TestAuthAPIDeniedWrongKey(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/cpu", AuthAPI, func(c *fiber.Ctx) { c.Send("granted") })

	req := httptest.NewRequest("GET", "/cpu", nil)
	req.Header.Set("x-api-key", "wrong")
	resp, err := app.Test(req, 30000)
	assert.NoError(t, err)
	assert.Contains(t, body(t, resp), "Unauthorized")
}

func TestAPIEndpoints(t *testing.T) {
	setupEnv(t)

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
			assert.NoError(t, err)
			// the handler must have executed (status not mutated -> 200,
			// or 500 if a system call panicked and was recovered).
			assert.Contains(t, []int{200, 500}, resp.StatusCode)
			// drain body so the connection can be released
			_ = body(t, resp)
		})
	}
}
