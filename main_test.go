package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

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
	// with arguments -> CLI runs (cmd.Execute) and dispatch returns true
	assert.True(t, dispatch([]string{"nitr", "version"}))
	// without arguments -> no CLI, dispatch returns false
	assert.False(t, dispatch([]string{"nitr"}))
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
