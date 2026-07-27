package handlers

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	rice "github.com/GeertJohan/go.rice"
	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func cdTemp(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	dir, err := ioutil.TempDir("", "nitrhandlertest")
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		_ = os.RemoveAll(dir)
	})
	return dir
}

// TestMain loads the view templates once for the entire handlers test suite.
// Doing it here (rather than a nil-check inside setupEnv) avoids a check-then-set
// race on ViewsBox the moment any test opts into t.Parallel().
func TestMain(m *testing.M) {
	ViewsBox = rice.MustFindBox("../app/views")
	os.Exit(m.Run())
}

// setupEnv provisions an isolated working dir with a database containing the
// default test user (password "123456", api key "testapikey"). View templates
// are loaded once by TestMain.
func setupEnv(t *testing.T) {
	t.Helper()
	cdTemp(t)
	require.NoError(t, db.SetupDB())
	require.NoError(t, db.SetUserData("1", models.User{
		Password: utils.PasswordHash("123456"),
		Apikey:   "testapikey",
	}))
}

// newTestApp returns a Fiber app with a panic-recovery middleware so a failing
// third-party call (e.g. the ISP endpoint hitting the network) cannot crash the
// whole test binary.
func newTestApp() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				c.Status(500).SendString("recovered")
			}
		}()
		return c.Next()
	})
	return app
}

func newRequest(method, target, formBody string, header http.Header) *http.Request {
	var body *strings.Reader
	if formBody != "" {
		body = strings.NewReader(formBody)
	}
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	return req
}

func get(t *testing.T, app *fiber.App, target string) *http.Response {
	t.Helper()
	resp, err := app.Test(newRequest("GET", target, "", nil), 30000)
	require.NoError(t, err)
	return resp
}

func post(t *testing.T, app *fiber.App, target, formBody string) *http.Response {
	t.Helper()
	resp, err := app.Test(newRequest("POST", target, formBody, nil), 30000)
	require.NoError(t, err)
	return resp
}

func getWithCookie(t *testing.T, app *fiber.App, target, cookie string) *http.Response {
	t.Helper()
	h := http.Header{}
	h.Set("Cookie", cookie)
	resp, err := app.Test(newRequest("GET", target, "", h), 30000)
	require.NoError(t, err)
	return resp
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

// sessionCookie extracts the "session_id=..." pair set in a Set-Cookie header.
func sessionCookie(resp *http.Response) string {
	for _, c := range resp.Header["Set-Cookie"] {
		if strings.HasPrefix(c, "session_id=") {
			return strings.SplitN(c, ";", 2)[0]
		}
	}
	return ""
}
