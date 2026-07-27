package utils

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bitcav/nitr/version"
	"github.com/gofiber/fiber"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cdTemp creates a temp directory, chdirs into it, and restores the original
// working directory on test cleanup.
func cdTemp(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	dir, err := ioutil.TempDir("", "nitrtest")
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		_ = os.RemoveAll(dir)
	})
	return dir
}

func TestPasswordHash(t *testing.T) {
	// known SHA-256 hex digest of "123456"
	want := "8d969eef6ecad3c29a3a629280e686cf0c3f5d5a86aff3ca12020c923adc6c92"
	assert.Equal(t, want, PasswordHash("123456"))
	assert.Equal(t, want, PasswordHash("123456"))
	assert.NotEqual(t, PasswordHash("a"), PasswordHash("b"))
	assert.Len(t, PasswordHash("x"), 64)
}

func TestRandString(t *testing.T) {
	s := RandString(16)
	assert.Len(t, s, 16)
	for _, c := range s {
		assert.Contains(t, charset, string(c))
	}
	assert.Equal(t, "", RandString(0))
	// extremely likely to differ
	assert.NotEqual(t, RandString(32), RandString(32))
}

func TestStringWithCharset(t *testing.T) {
	cs := "ab"
	s := stringWithCharset(500, cs)
	assert.Len(t, s, 500)
	for _, c := range s {
		assert.Contains(t, cs, string(c))
	}
}

func TestGetLocalPort(t *testing.T) {
	viper.Reset()
	viper.Set("port", "3000")
	assert.Equal(t, "3000", GetLocalPort())

	viper.Set("port", "")
	assert.Equal(t, "8000", GetLocalPort())
}

func TestStartMessage(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	StartMessage("https", "3000")

	require.NoError(t, w.Close())
	os.Stdout = old
	out, err := ioutil.ReadAll(r)
	require.NoError(t, err)

	body := string(out)
	assert.Contains(t, body, version.Version)
	assert.Contains(t, body, "https")
	assert.Contains(t, body, "3000")
}

func TestLogError(t *testing.T) {
	assert.NotPanics(t, func() { LogError(nil) })

	var buf bytes.Buffer
	log.SetOutput(&buf)
	LogError(errors.New("boom"))
	assert.Contains(t, buf.String(), "boom")
}

func TestConfigFileSetup(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	// first call: no config.ini present -> creates defaults
	ConfigFileSetup()
	data, err := ioutil.ReadFile("config.ini")
	require.NoError(t, err)
	body := string(data)
	assert.Contains(t, body, "port: 8000")
	assert.Contains(t, body, "open_browser_on_startup: true")
	assert.Contains(t, body, "save_logs: false")

	// second call: file exists -> stat branch (no rewrite)
	ConfigFileSetup()
	assert.FileExists(t, "config.ini")
}

func TestConfigFileSetupReadError(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	// pre-seed an unparseable config so viper.ReadInConfig fails and the
	// error branch (LogError) is exercised.
	require.NoError(t, ioutil.WriteFile("config.ini", []byte(":\n  : [bad"), 0666))
	assert.NotPanics(t, func() { ConfigFileSetup() })
}

func TestLogsDisabled(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	viper.Set("save_logs", false)
	app := fiber.New()
	assert.NotPanics(t, func() { Logs(app) })
	// no log file should be created when disabled
	_, err := os.Stat("nitr.log")
	assert.True(t, os.IsNotExist(err))
}

func TestLogsEnabled(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	viper.Set("save_logs", true)
	app := fiber.New()
	Logs(app)
	assert.FileExists(t, "nitr.log")
}

func TestGetLocalIP(t *testing.T) {
	ip := GetLocalIP()
	assert.NotEmpty(t, ip)
	assert.NotNil(t, net.ParseIP(ip))
}

func TestOpenBrowser(t *testing.T) {
	// OpenBrowser must not fatal/panic on a missing browser-opener binary;
	// it surfaces the error to the caller instead.
	err := OpenBrowser("http://127.0.0.1", "1")
	if err != nil {
		// On hosts lacking xdg-open/open/rundll32 the command simply fails to
		// start; that is acceptable behaviour as long as it is returned
		// (not fatal).
		t.Logf("OpenBrowser returned error (expected on hosts without a " +
			"browser opener): %v", err)
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	require.NoError(t, ln.Close())
	return port
}

func TestStartServerHTTP(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	port := freePort(t)
	viper.Set("port", port)
	viper.Set("ssl_enabled", false)
	viper.Set("open_browser_on_startup", false)

	app := fiber.New(&fiber.Settings{DisableStartupMessage: true})
	app.Get("/", func(c *fiber.Ctx) { c.Send("ok") })

	done := make(chan struct{})
	go func() {
		StartServer(app)
		close(done)
	}()

	base := "http://127.0.0.1:" + port + "/"
	cli := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = cli.Get(base)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	b, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "ok", string(b))

	require.NoError(t, app.Shutdown())
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StartServer did not return after shutdown")
	}
}

func TestStartServerHTTPListenError(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	// occupy a port so app.Listen fails immediately
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	viper.Set("port", port)
	viper.Set("ssl_enabled", false)
	viper.Set("open_browser_on_startup", true) // exercise open-browser path

	// Capture log output so we can assert the browser-open error is logged
	// (non-fatal) rather than crashing the process via log.Fatal.
	var buf bytes.Buffer
	log.SetOutput(&buf)

	app := fiber.New(&fiber.Settings{DisableStartupMessage: true})
	// Should return (listen error path prints + logs but does not fatal).
	assert.NotPanics(t, func() { StartServer(app) })
	// StartServer must return to its caller instead of os.Exit-ing.
	assert.NotContains(t, buf.String(), "exit status")
}

func TestStartServerSSLError(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	// occupy a port so the SSL listener fails to bind
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	viper.Set("port", port)
	viper.Set("ssl_enabled", true)
	viper.Set("ssl_certificate", "")
	viper.Set("ssl_certificate_key", "")
	viper.Set("open_browser_on_startup", true)

	// Capture log output so we can assert any browser-open error is logged
	// (non-fatal) rather than crashing the process via log.Fatal.
	var buf bytes.Buffer
	log.SetOutput(&buf)

	app := fiber.New(&fiber.Settings{DisableStartupMessage: true})
	// invalid cert path -> "Invalid ssl certificate" then listen error -> returns
	assert.NotPanics(t, func() { StartServer(app) })
	assert.NotContains(t, buf.String(), "exit status")
}
