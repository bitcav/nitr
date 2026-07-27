package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/bitcav/nitr/utils"
	"github.com/bitcav/nitr/version"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cdTempMain(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	dir, err := ioutil.TempDir("", "nitrmaintest")
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		_ = os.RemoveAll(dir)
	})
	return dir
}

// freeListener binds an ephemeral port and returns the held listener plus its
// string form. The listener stays open so the OS cannot hand the same port to
// another process between check and use: bind-close-rebind is a TOCTOU race
// across the separate OS processes that `go test ./...` spawns per package
// (the ephemeral port space is OS-global). Tests that actually start a server
// hand the listener to it via utils.ListenFunc (which closes it on shutdown);
// closing an already-closed listener is a benign no-op whose error we ignore.
func freeListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return ln, strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

// TestProgramStop covers the nil-app path of Stop (a program that was never
// started has nothing to shut down); the shutdown path is exercised by
// TestServerBootsViaProgramStart.
func TestProgramStop(t *testing.T) {
	p := &program{}
	assert.NoError(t, p.Stop(nil))
}

func TestDispatchCLI(t *testing.T) {
	cdTempMain(t)
	// with arguments -> CLI runs the version subcommand and prints the
	// real version string; asserting on captured output is what proves
	// args actually reach cobra (a boolean-only check would pass either way).
	out := captureStdout(t, func() {
		assert.True(t, dispatch([]string{"nitr", "version"}))
	})
	assert.Contains(t, out, "Nitr v"+version.Version)
	// without arguments -> no CLI, dispatch returns false
	assert.False(t, dispatch([]string{"nitr"}))
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything fn wrote. fmt.Printf in cmd.VersionCmd targets os.Stdout
// directly, so this is the seam that makes the routing test meaningful.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	require.NoError(t, w.Close())
	b, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}

func TestInitService(t *testing.T) {
	s, lg, err := initService()
	require.NoError(t, err)
	assert.NotNil(t, s)
	assert.NotNil(t, lg)
}

// TestServerBootsViaProgramStart exercises the full server bootstrap
// (program.Start -> server -> all route/middleware registration ->
// utils.StartServer). Setup now runs synchronously inside Start and only the
// blocking listener is backgrounded, so the app is ready to answer by the
// time Start returns; we still poll to tolerate listener ramp-up.
//
// The listener is held open (not closed-and-rebound) and handed to the server
// via utils.ListenFunc, closing the TOCTOU window that two `go test ./...`
// package binaries open when each frees a port and rebinds it against a
// global ephemeral port space. p.Stop (registered as the LAST cleanup, so it
// runs FIRST under LIFO) must run before cdTempMain's cleanup restores cwd
// and deletes the temp dir, otherwise the server goroutine would keep
// resolving nitr.db/config.ini against whatever directory is current.
func TestServerBootsViaProgramStart(t *testing.T) {
	dir := cdTempMain(t)
	ln, port := freeListener(t)

	// Pre-seed config.ini so ConfigFileSetup does not overwrite our values and
	// so StartServer uses a known free port without spawning a browser.
	cfg := fmt.Sprintf(
		"port: %s\nopen_browser_on_startup: false\nsave_logs: false\nssl_enabled: false\n",
		port,
	)
	require.NoError(t, ioutil.WriteFile("config.ini", []byte(cfg), 0666))

	p := &program{}
	// Serve on the held listener so there is no close-rebind window for
	// another package's test binary to race on. fiber.Shutdown closes ln.
	origListen := utils.ListenFunc
	utils.ListenFunc = func(app *fiber.App, addr string) error { return app.Listener(ln) }
	t.Cleanup(func() { utils.ListenFunc = origListen })

	// Start runs setup synchronously and backgrounds only the blocking
	// listener, so it returns once the app is assembled.
	require.NoError(t, p.Start(nil))

	// t.Cleanup is LIFO: this was registered AFTER cdTempMain's cleanup, so
	// it runs BEFORE cwd is restored and the temp dir deleted — exactly the
	// order a leaked server goroutine would corrupt if reversed.
	t.Cleanup(func() {
		assert.NoError(t, p.Stop(nil))
		// Demonstrate the listener is gone: rebinding the same address must
		// succeed because fiber.Shutdown closed it. A leaked server would
		// hold the port and this bind would fail with "address in use".
		probe, err := net.Listen("tcp", ln.Addr().String())
		if !assert.NoError(t, err, "port %s still in use after Stop — listener leaked", port) {
			return
		}
		probe.Close()
	})

	base := "http://127.0.0.1:" + port + "/"
	cli := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}

	var resp *http.Response
	var err error
	for i := 0; i < 100; i++ {
		resp, err = cli.Get(base)
		if err == nil {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	b, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	assert.NotEmpty(t, b) // the login view HTML

	_ = dir
}

// TestStartPropagatesLogsError is the regression guard for the
// fire-and-forget-goroutine defect: when save_logs=true and nitr.log cannot
// be opened, utils.Logs used to call log.Fatalf from inside the goroutine
// spawned by Start, os.Exit-ing the whole process and bypassing main's
// error-reporting path (if err := s.Run(); err != nil). The fix makes
// utils.Logs return an error that Start propagates.
//
// We force the failing path with save_logs=true and a directory at the
// "nitr.log" path, so os.OpenFile fails. Before the fix this test would
// crash the binary via log.Fatalf (run in isolation to observe that); now
// the failure must surface as a returned error. The guard is structured to
// observe the error rather than let log.Fatalf kill the test run.
func TestStartPropagatesLogsError(t *testing.T) {
	cdTempMain(t)
	// This test never reaches ListenFunc (Start errors inside Logs), so it
	// takes no part in the close-rebind TOCTOU race; closing the held
	// listener immediately returns its port to the ephemeral pool.
	ln, port := freeListener(t)
	ln.Close()

	// A directory where the file should be makes os.OpenFile fail
	// (EISDIR on Linux, the equivalent error on Windows), which is the
	// exact failure mode the old log.Fatalf guarded.
	require.NoError(t, os.Mkdir("nitr.log", 0755))

	cfg := fmt.Sprintf(
		"port: %s\nopen_browser_on_startup: false\nsave_logs: true\nssl_enabled: false\n",
		port,
	)
	require.NoError(t, ioutil.WriteFile("config.ini", []byte(cfg), 0666))

	p := &program{}
	err := p.Start(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nitr.log")
}

// TestBuiltBinaryRuns guards against the go.rice/go.zipexe init() panic class
// of bug: every compiled nitr binary panicked before main ran on Go 1.26 ELF,
// while the whole test suite stayed green because tests call rice.MustFindBox
// in-process from the source tree and never touch the appended-zip path. This
// test builds the real binary and executes it, so a regression of that init()
// panic surfaces as a non-zero exit (with the panic on stderr) instead of
// shipping green. It runs the binary from t.TempDir() for defensive isolation,
// keeping any cwd-relative I/O out of the repo.
func TestBuiltBinaryRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not available, skipping binary build test: %v", err)
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "nitr")
	if runtime.GOOS == "windows" {
		bin += ".exe" // Windows cannot exec a binary without the .exe suffix
	}

	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	run := exec.Command(bin, "version")
	run.Dir = tmp // defensive isolation: keep any cwd-relative I/O out of the repo
	out, err := run.CombinedOutput()
	if err != nil {
		// Distinguish "the binary never ran at all" (e.g. wrong name /
		// missing .exe on Windows, ENOEXEC) from "it ran and exited
		// non-zero" (the init()-panic class this test exists to catch).
		if _, notStarted := err.(*exec.Error); notStarted {
			t.Fatalf("built `nitr version` could not be executed at all: %v\n%s", err, out)
		}
		t.Fatalf("built `nitr version` ran but exited non-zero (init panic?): %v\n%s", err, out)
	}
	assert.Contains(t, string(out), "Nitr v")
}
