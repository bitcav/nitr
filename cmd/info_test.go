package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Vendor string  `json:"vendor"`
	Usage  float64 `json:"usage"`
}

func TestRunInfoOnceJSONMatchesJSONMarshalExactly(t *testing.T) {
	var buf bytes.Buffer
	r := infoResource{name: "cpu", fetch: func() (any, error) { return sample{Vendor: "Intel", Usage: 4.5}, nil }}

	require.NoError(t, runInfoOnce(&buf, r, true))

	want, err := json.Marshal(sample{Vendor: "Intel", Usage: 4.5})
	require.NoError(t, err)
	// Byte-identical to json.Marshal -- what fiber's c.JSON uses -- with no
	// trailing newline appended, so `nitr cpu --json` is the same body an
	// API client would receive from GET /api/v1/cpu.
	assert.Equal(t, string(want), buf.String())
	assert.False(t, bytes.HasSuffix(buf.Bytes(), []byte("\n")))
}

func TestRunInfoOnceHumanReadable(t *testing.T) {
	var buf bytes.Buffer
	r := infoResource{name: "cpu", fetch: func() (any, error) { return sample{Vendor: "Intel", Usage: 4.5}, nil }}

	require.NoError(t, runInfoOnce(&buf, r, false))
	assert.Contains(t, buf.String(), "Vendor:")
	assert.Contains(t, buf.String(), "Intel")
}

// TestRunInfoOnceFetchErrorNonPrivileged proves a plain collector error
// (not a permission problem, and not one of the privileged resources)
// surfaces as-is, without the "elevated privileges" hint that would
// mislead the user into trying `sudo` for an unrelated failure.
func TestRunInfoOnceFetchErrorNonPrivileged(t *testing.T) {
	var buf bytes.Buffer
	r := infoResource{name: "isp", fetch: func() (any, error) { return nil, errors.New("timed out") }}

	err := runInfoOnce(&buf, r, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "isp")
	assert.Contains(t, err.Error(), "timed out")
	assert.NotContains(t, err.Error(), "elevated privileges")
}

// TestRunInfoOncePermissionErrorOnPrivilegedResource is memory's real
// shape: memdev.Info() returns a genuine fs.ErrPermission-flavored error
// (unlike chassis/baseboard/product, which return an empty struct with no
// error). That must map to the "requires elevated privileges" hint and a
// non-zero exit (an error return), not the empty-struct rendering path.
func TestRunInfoOncePermissionErrorOnPrivilegedResource(t *testing.T) {
	var buf bytes.Buffer
	r := infoResource{
		name:       "memory",
		privileged: true,
		fetch:      func() (any, error) { return nil, fmt.Errorf("open /sys/firmware/dmi/tables: %w", os.ErrPermission) },
	}

	err := runInfoOnce(&buf, r, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires elevated privileges")
	assert.Contains(t, err.Error(), "sudo nitr memory")
}

type zeroStruct struct {
	Vendor string `json:"vendor"`
	Serial string `json:"serial"`
}

// TestRunInfoOncePrivilegedZeroValueHumanMode is chassis/baseboard/product's
// shape: ghw fails silently inside nitr-core (logged, not returned), so the
// collector "succeeds" with a zero-value struct. Human-readable mode must
// swap in the privilege hint instead of printing an empty table.
func TestRunInfoOncePrivilegedZeroValueHumanMode(t *testing.T) {
	var buf bytes.Buffer
	r := infoResource{name: "chassis", privileged: true, fetch: func() (any, error) { return zeroStruct{}, nil }}

	require.NoError(t, runInfoOnce(&buf, r, false))
	assert.Contains(t, buf.String(), "requires elevated privileges")
	assert.Contains(t, buf.String(), "sudo nitr chassis")
}

// TestRunInfoOncePrivilegedZeroValueJSONMode proves --json's contract
// ("byte-identical to the matching API response") holds even for the
// privileged-empty case: the API has no concept of this hint, it just
// returns the zero-value struct as 200 JSON, so the CLI must too.
func TestRunInfoOncePrivilegedZeroValueJSONMode(t *testing.T) {
	var buf bytes.Buffer
	r := infoResource{name: "chassis", privileged: true, fetch: func() (any, error) { return zeroStruct{}, nil }}

	require.NoError(t, runInfoOnce(&buf, r, true))
	want, err := json.Marshal(zeroStruct{})
	require.NoError(t, err)
	assert.Equal(t, string(want), buf.String())
	assert.NotContains(t, buf.String(), "privileges")
}

// TestRunInfoOncePrivilegedNonZeroHumanMode proves a privileged resource
// that returns SOME data (chassis vendor readable, serial not, as observed
// on a real unprivileged Linux host) renders normally rather than being
// blanked out by the hint -- the zero-value check must look at the whole
// struct, not fire on any one empty field.
func TestRunInfoOncePrivilegedNonZeroHumanMode(t *testing.T) {
	var buf bytes.Buffer
	r := infoResource{name: "chassis", privileged: true, fetch: func() (any, error) { return zeroStruct{Vendor: "LENOVO"}, nil }}

	require.NoError(t, runInfoOnce(&buf, r, false))
	assert.Contains(t, buf.String(), "LENOVO")
	assert.NotContains(t, buf.String(), "elevated privileges")
}

func TestNewInfoCmdJSONFlag(t *testing.T) {
	r := infoResource{name: "cpu", fetch: func() (any, error) { return sample{Vendor: "Intel"}, nil }}
	cmd := newInfoCmd(r)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Flags().Set("json", "true"))

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Equal(t, `{"vendor":"Intel","usage":0}`, buf.String())
}

func TestNewInfoCmdInvalidWatchDuration(t *testing.T) {
	r := infoResource{name: "cpu", fetch: func() (any, error) { return sample{}, nil }}
	cmd := newInfoCmd(r)
	require.NoError(t, cmd.Flags().Set("watch", "not-a-duration"))

	err := cmd.RunE(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --watch interval")
}

// TestNewInfoCmdWatchBareFlagDefaultsTo2s proves `-w` with no value (cobra's
// NoOptDefVal) is accepted as a valid duration rather than erroring, since
// the ticket's flag is documented as "--watch / -w [interval]" with an
// implied default when no value is given.
func TestNewInfoCmdWatchBareFlagDefaultsTo2s(t *testing.T) {
	r := infoResource{name: "cpu", fetch: func() (any, error) { return sample{}, nil }}
	cmd := newInfoCmd(r)
	require.NoError(t, cmd.ParseFlags([]string{"-w"}))
	watch, err := cmd.Flags().GetString("watch")
	require.NoError(t, err)
	assert.Equal(t, "2s", watch)
}

// TestRunInfoWatchStopsOnContextCancel proves the watch loop actually exits
// when its context is cancelled (the Ctrl+C path in production, via
// signal.NotifyContext) instead of looping forever, and that it re-fetches
// more than once -- i.e. it is really polling, not rendering a single
// snapshot repeatedly from a cache.
func TestRunInfoWatchStopsOnContextCancel(t *testing.T) {
	var calls int32
	r := infoResource{name: "cpu", fetch: func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return sample{}, nil
	}}

	ctx, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		runInfoWatch(ctx, &buf, r, true, time.Millisecond)
		close(done)
	}()

	require.Eventually(t, func() bool { return atomic.LoadInt32(&calls) >= 2 }, time.Second, time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runInfoWatch did not return after context cancellation")
	}
}

// TestRunInfoWatchSurvivesPerTickError proves a transient fetch failure
// during --watch is reported and the loop keeps polling, rather than the
// whole command dying on the first error the way a one-shot invocation
// would (and should) -- a monitoring loop that exits on its first hiccup
// isn't useful.
func TestRunInfoWatchSurvivesPerTickError(t *testing.T) {
	var calls int32
	r := infoResource{name: "isp", fetch: func() (any, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return nil, errors.New("transient failure")
		}
		return sample{}, nil
	}}

	ctx, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		runInfoWatch(ctx, &buf, r, true, time.Millisecond)
		close(done)
	}()

	require.Eventually(t, func() bool { return atomic.LoadInt32(&calls) >= 2 }, time.Second, time.Millisecond)
	cancel()
	<-done
}

func TestISPInfoWithTimeoutFires(t *testing.T) {
	start := time.Now()
	_, err := ispInfoWithTimeout(time.Nanosecond)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Less(t, elapsed, 2*time.Second)
}

// TestInfoResourcesNoReservedNameCollisions guards the command surface
// against reusing a name the ticket explicitly reserves for something else
// (server/version/key/passwd/qr/install/uninstall/start/stop/status).
func TestInfoResourcesNoReservedNameCollisions(t *testing.T) {
	reserved := map[string]bool{
		"server": true, "version": true, "key": true, "passwd": true, "qr": true,
		"install": true, "uninstall": true, "start": true, "stop": true,
		"restart": true, "status": true,
	}
	seen := map[string]bool{}
	for _, r := range infoResources {
		assert.False(t, reserved[r.name], "%q is a reserved command name", r.name)
		assert.False(t, seen[r.name], "%q registered twice", r.name)
		seen[r.name] = true
	}
}

// TestInfoCommandsRegisteredOnRoot proves buildInfoCmds's output actually
// reaches rootCmd (root.go's init), not just that the commands can be built
// in isolation.
func TestInfoCommandsRegisteredOnRoot(t *testing.T) {
	names := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}
	for _, r := range infoResources {
		assert.True(t, names[r.name], "%q not registered on rootCmd", r.name)
	}
}
