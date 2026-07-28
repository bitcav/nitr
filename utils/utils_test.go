package utils

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bitcav/nitr/version"
	"github.com/gofiber/fiber/v2"
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

// TestRandStringConcurrent guards the data race on the old shared
// *rand.Rand source: it fails under -race with the pre-v2 implementation.
func TestRandStringConcurrent(t *testing.T) {
	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			s := RandString(10)
			assert.Len(t, s, 10)
		}()
	}
	wg.Wait()
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

// TestConfigEnvOverride proves the env layer works end-to-end: with
// AutomaticEnv and the NITR prefix set by ConfigFileSetup, NITR_PORT beats
// the port written in config.ini. Underscore keys need no replacer —
// NITR_OPEN_BROWSER_ON_STARTUP maps to open_browser_on_startup as-is.
func TestConfigEnvOverride(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	require.NoError(t, ioutil.WriteFile("config.ini",
		[]byte("port: 1111\nopen_browser_on_startup: false\n"), 0666))
	t.Setenv("NITR_PORT", "2222")
	t.Setenv("NITR_OPEN_BROWSER_ON_STARTUP", "true")

	ConfigFileSetup()
	assert.Equal(t, "2222", GetLocalPort(), "NITR_PORT must override the config file")
	assert.True(t, viper.GetBool("open_browser_on_startup"),
		"NITR_OPEN_BROWSER_ON_STARTUP must override the config file without a key replacer")
}

// TestConfigFlagPath covers the --config key: it points ConfigFileSetup at
// an arbitrary path, a default file is created there when missing, and its
// values are the ones read back.
func TestConfigFlagPath(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	require.NoError(t, os.Mkdir("conf", 0755))
	custom := filepath.Join("conf", "custom.ini")
	viper.Set("config", custom)

	ConfigFileSetup()
	assert.FileExists(t, custom)
	assert.Equal(t, "8000", GetLocalPort(), "values must be read from the --config file")

	// A value edited in the custom file is picked up on the next load,
	// proving the read also honours --config rather than ./config.ini.
	require.NoError(t, ioutil.WriteFile(custom, []byte("port: 9999\n"), 0666))
	ConfigFileSetup()
	assert.Equal(t, "9999", GetLocalPort())
}

// TestConfigZeroConfigPath is the regression guard for the zero-config
// path: no flags, no env, no existing file — a default config.ini is
// created in the cwd and port resolves to 8000, exactly as before flags
// existed.
func TestConfigZeroConfigPath(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	ConfigFileSetup()
	assert.FileExists(t, "config.ini")
	assert.Equal(t, "8000", GetLocalPort())
	body, err := ioutil.ReadFile("config.ini")
	require.NoError(t, err)
	// The generated header must warn that the syntax is YAML, not INI.
	assert.Contains(t, string(body), "Parsed as YAML despite the .ini extension")
	assert.Contains(t, string(body), "bind_address: 0.0.0.0")
}

func TestBindAddress(t *testing.T) {
	viper.Reset()
	assert.Equal(t, "0.0.0.0", BindAddress(), "default must preserve listen-on-all-interfaces")
	viper.Set("bind_address", "127.0.0.1")
	assert.Equal(t, "127.0.0.1", BindAddress())
}

func TestLogsDisabled(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	viper.Set("save_logs", false)
	app := fiber.New()
	assert.NotPanics(t, func() {
		err := Logs(app)
		assert.NoError(t, err)
	})
	// no log file should be created when disabled
	_, err := os.Stat("nitr.log")
	assert.True(t, os.IsNotExist(err))
}

func TestLogsEnabled(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	viper.Set("save_logs", true)
	app := fiber.New()
	require.NoError(t, Logs(app))
	assert.FileExists(t, "nitr.log")
}

// TestLogPath proves nitr.log follows the same data_dir resolution as
// database.DBPath: bare name in the cwd by default, joined under data_dir
// when set. This is the regression guard for the bug that had /ready
// ignoring --data-dir — Logs is the second caller of the same pattern and
// would have drifted identically.
func TestLogPath(t *testing.T) {
	viper.Reset()
	assert.Equal(t, "nitr.log", LogPath(), "no data_dir must keep nitr.log in cwd")

	viper.Set("data_dir", "/var/lib/nitr")
	assert.Equal(t, filepath.Join("/var/lib/nitr", "nitr.log"), LogPath())

	viper.Set("data_dir", "data")
	assert.Equal(t, filepath.Join("data", "nitr.log"), LogPath())
}

// TestLogsEnabledDataDir proves that with save_logs and data_dir both set,
// nitr.log is created under data_dir and nothing is written to the cwd —
// the actual filesystem behaviour, not just the resolved string.
func TestLogsEnabledDataDir(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	viper.Set("save_logs", true)
	viper.Set("data_dir", "data")
	app := fiber.New()
	require.NoError(t, Logs(app))

	assert.FileExists(t, filepath.Join("data", "nitr.log"))
	_, err := os.Stat("nitr.log")
	assert.True(t, os.IsNotExist(err), "with data_dir set, nothing may be written to the cwd")
}

func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	// On a host with no default route the dial fails; the binding requirement
	// is that the error comes back to the caller instead of killing the
	// process, so both outcomes are acceptable here.
	if err != nil {
		assert.Empty(t, ip)
		return
	}
	assert.NotEmpty(t, ip)
	assert.NotNil(t, net.ParseIP(ip))
}

func TestOpenBrowser(t *testing.T) {
	// OpenBrowser spawns a real OS browser process; skip it during normal
	// `go test ./...` runs. Opt in explicitly with NITR_TEST_REAL_BROWSER=1
	// to exercise the real xdg-open/open/rundll32 path.
	if os.Getenv("NITR_TEST_REAL_BROWSER") != "1" {
		t.Skip("skipping real browser open; set NITR_TEST_REAL_BROWSER=1 to run")
	}

	// OpenBrowser must not fatal/panic on a missing browser-opener binary;
	// it surfaces the error to the caller instead.
	err := OpenBrowser("http://127.0.0.1", "1")
	if err != nil {
		// On hosts lacking xdg-open/open/rundll32 the command simply fails to
		// start; that is acceptable behaviour as long as it is returned
		// (not fatal).
		t.Logf("OpenBrowser returned error (expected on hosts without a "+
			"browser opener): %v", err)
	}
}

// freeListener binds an ephemeral port and returns the held listener plus its
// string form. The listener stays open so the OS cannot hand the same port to
// another process between check and use: bind-close-rebind is a TOCTOU race
// across the separate OS processes that `go test ./...` spawns per package.
// The caller either hands the listener to the server (via ListenFunc, which
// closes it on shutdown) or closes it itself; closing an already-closed
// listener is a benign no-op whose error we ignore.
func freeListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return ln, strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func TestStartServerHTTP(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	ln, port := freeListener(t)
	viper.Set("port", port)
	viper.Set("ssl_enabled", false)
	viper.Set("open_browser_on_startup", false)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// Serve on the held listener so there is no close-rebind window for
	// another package's test binary to race on. fiber.Shutdown closes ln.
	origListen := ListenFunc
	ListenFunc = func(a *fiber.App, addr string) error { return a.Listener(ln) }
	t.Cleanup(func() { ListenFunc = origListen })

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

	// This test never actually binds (ListenFunc is stubbed to error), so it
	// takes no part in the close-rebind TOCTOU race; closing the held
	// listener immediately returns its port to the ephemeral pool.
	ln, port := freeListener(t)
	ln.Close()
	viper.Set("port", port)
	viper.Set("ssl_enabled", false)
	viper.Set("open_browser_on_startup", true) // exercise open-browser path

	// Stub the browser opener so the test never spawns a real process; the
	// test only asserts that an error is logged non-fatally, so a canned
	// error exercises exactly the same code path.
	origOpen := openBrowserFunc
	openBrowserFunc = func(string, string) error { return errors.New("stub: browser disabled") }
	t.Cleanup(func() { openBrowserFunc = origOpen })

	// Force StartServer down its listen-error path via the seam rather than
	// relying on a double-bind to fail: Windows socket semantics (SO_REUSEADDR
	// vs. SO_EXCLUSIVEADDRUSE) permit a second bind that Linux refuses, so a
	// real second bind is not a portable way to provoke a listen error and
	// would hang this test on Windows. Stubbing ListenFunc makes the failure
	// deterministic and bounded to seconds.
	origListen := ListenFunc
	ListenFunc = func(*fiber.App, string) error { return errors.New("stub: listen disabled") }
	t.Cleanup(func() { ListenFunc = origListen })

	// Capture log output so we can assert the browser-open error is logged
	// (non-fatal) rather than crashing the process via os.Exit.
	var buf bytes.Buffer
	log.SetOutput(&buf)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	// Should return (listen error path prints + logs but does not fatal).
	assert.NotPanics(t, func() { StartServer(app) })
	// StartServer must return to its caller instead of os.Exit-ing.
	assert.NotContains(t, buf.String(), "exit status")
}

// TestStartServerBindAddress proves the address handed to the listener is
// bind_address:port — 0.0.0.0 by default (all interfaces, as before), and
// the configured value when bind_address is set (e.g. --host 127.0.0.1).
func TestStartServerBindAddress(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	viper.Set("port", "1")
	viper.Set("ssl_enabled", false)
	viper.Set("open_browser_on_startup", false)

	var gotAddr string
	origListen := ListenFunc
	ListenFunc = func(_ *fiber.App, addr string) error {
		gotAddr = addr
		return errors.New("stub: stop after capturing addr")
	}
	t.Cleanup(func() { ListenFunc = origListen })

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	StartServer(app)
	assert.Equal(t, "0.0.0.0:1", gotAddr, "unset bind_address must listen on all interfaces")

	viper.Set("bind_address", "127.0.0.1")
	StartServer(app)
	assert.Equal(t, "127.0.0.1:1", gotAddr)
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

	// Stub the browser opener so the test never spawns a real process.
	origOpen := openBrowserFunc
	openBrowserFunc = func(string, string) error { return errors.New("stub: browser disabled") }
	t.Cleanup(func() { openBrowserFunc = origOpen })

	// Capture log output so we can assert any browser-open error is logged
	// (non-fatal) rather than crashing the process via os.Exit.
	var buf bytes.Buffer
	log.SetOutput(&buf)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	// invalid cert path -> "Invalid ssl certificate" then listen error -> returns
	assert.NotPanics(t, func() { StartServer(app) })
	assert.NotContains(t, buf.String(), "exit status")
}
