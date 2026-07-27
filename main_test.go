package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/bitcav/nitr/version"
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

func freePortStr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	require.NoError(t, ln.Close())
	return port
}

// TestProgramStop covers the trivial no-op Stop handler.
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
// (program.Start -> run -> server -> all route/middleware registration ->
// utils.StartServer). The blocking listener runs in a background goroutine
// launched by Start; we verify the server answers requests, then leave the
// goroutine to be reaped when the test process exits.
func TestServerBootsViaProgramStart(t *testing.T) {
	dir := cdTempMain(t)
	port := freePortStr(t)

	// Pre-seed config.ini so ConfigFileSetup does not overwrite our values and
	// so StartServer uses a known free port without spawning a browser.
	cfg := fmt.Sprintf(
		"port: %s\nopen_browser_on_startup: false\nsave_logs: false\nssl_enabled: false\n",
		port,
	)
	require.NoError(t, ioutil.WriteFile("config.ini", []byte(cfg), 0666))

	p := &program{}
	// Start spawns run() -> server() in a goroutine and returns immediately.
	require.NoError(t, p.Start(nil))

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

// TestBuiltBinaryRuns guards against the go.rice/go.zipexe init() panic class
// of bug: every compiled nitr binary panicked before main ran on Go 1.26 ELF,
// while the whole test suite stayed green because tests call rice.MustFindBox
// in-process from the source tree and never touch the appended-zip path. This
// test builds the real binary and executes it, so a regression of that init()
// panic surfaces as a non-zero exit (with the panic on stderr) instead of
// shipping green. It runs in t.TempDir() because the binary writes a database
// and log on startup and must not litter the repo.
func TestBuiltBinaryRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not available, skipping binary build test: %v", err)
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "nitr")

	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	run := exec.Command(bin, "version")
	run.Dir = tmp // startup writes config.ini/db/log here, not in the repo
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("built `nitr version` exited non-zero (init panic?): %v\n%s", err, out)
	}
	assert.Contains(t, string(out), "Nitr v")
}
