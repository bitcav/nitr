package cmd

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeService is an in-memory service.Service for hermetic lifecycle tests.
// Each method records its call and returns the configured error, so tests can
// drive install/uninstall/start/stop/status without touching the host service
// manager (which would need root and mutate /etc/systemd).
type fakeService struct {
	installErr   error
	uninstallErr error
	startErr     error
	stopErr      error
	status       service.Status
	statusErr    error

	installed   bool
	uninstalled bool
	started     bool
	stopped     bool
}

func (f *fakeService) Run() error                  { return nil }
func (f *fakeService) Start() error                { f.started = true; return f.startErr }
func (f *fakeService) Stop() error                 { f.stopped = true; return f.stopErr }
func (f *fakeService) Restart() error              { return nil }
func (f *fakeService) Install() error              { f.installed = true; return f.installErr }
func (f *fakeService) Uninstall() error            { f.uninstalled = true; return f.uninstallErr }
func (f *fakeService) Logger(chan<- error) (service.Logger, error) { return nil, nil }
func (f *fakeService) SystemLogger(chan<- error) (service.Logger, error) { return nil, nil }
func (f *fakeService) String() string              { return ServiceName }
func (f *fakeService) Platform() string            { return "fake-test-platform" }
func (f *fakeService) Status() (service.Status, error) {
	return f.status, f.statusErr
}

// withFakeService swaps newServiceFunc for one returning svc for the duration
// of fn, restoring it afterwards so other tests keep using the real factory.
func withFakeService(t *testing.T, svc service.Service, fn func()) {
	t.Helper()
	orig := newServiceFunc
	newServiceFunc = func() (service.Service, error) { return svc, nil }
	t.Cleanup(func() { newServiceFunc = orig })
	fn()
}

// runRoot executes args against the cobra tree, returning combined stdout and
// the error from Execute. Unlike ExecuteArgs it does not os.Exit on error, so
// the lifecycle commands' non-zero paths can be asserted on directly.
func runRoot(t *testing.T, args []string) (string, error) {
	t.Helper()
	// cobra hangs the --help flag on each (shared) command instance; once a
	// prior Execute set it true it stays true across tests, because Parse on
	// a fresh argv does not reset an already-set value. A later bare "status"
	// run would then be mistaken for "status -h" and print help instead of
	// running the command. Reset help across the tree before every run.
	resetHelpFlags(rootCmd)
	rootCmd.SetArgs(args)

	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldOut := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = oldOut }()

	type res struct {
		b   []byte
		err error
	}
	ch := make(chan res, 1)
	go func() {
		b, _ := io.ReadAll(r)
		ch <- res{b, nil}
	}()

	runErr := rootCmd.Execute()
	require.NoError(t, w.Close())
	out := (<-ch).b
	return string(out), runErr
}

// resetHelpFlags clears any lingering --help=true on cmd and its subcommands.
// See runRoot for why this is necessary when the command tree is shared
// across tests.
func resetHelpFlags(cmd *cobra.Command) {
	if hf := cmd.Flags().Lookup("help"); hf != nil {
		_ = hf.Value.Set("false")
	}
	for _, c := range cmd.Commands() {
		resetHelpFlags(c)
	}
}

// TestSideEffectGuardIsNotVacuous proves the provisioningSideEffectsPresent
// detector (and therefore assertNoProvisioningSideEffects) actually detects
// files. A guard that passes vacuously is the failure mode the project has
// been bitten by twice: if this ever returns false even when the files exist,
// every "no side effects" test below would pass for the wrong reason.
func TestSideEffectGuardIsNotVacuous(t *testing.T) {
	cdTemp(t)

	// Empty dir: nothing present.
	present, name := provisioningSideEffectsPresent()
	assert.False(t, present, "guard should report clean on an empty dir")

	// config.ini created -> detected.
	require.NoError(t, os.WriteFile("config.ini", []byte("x"), 0644))
	present, name = provisioningSideEffectsPresent()
	assert.True(t, present, "guard must detect config.ini")
	assert.Equal(t, "config.ini", name)

	// nitr.db also detected once config.ini is gone.
	require.NoError(t, os.Remove("config.ini"))
	require.NoError(t, os.WriteFile("nitr.db", []byte("x"), 0644))
	present, name = provisioningSideEffectsPresent()
	assert.True(t, present, "guard must detect nitr.db")
	assert.Equal(t, "nitr.db", name)

	// Back to clean.
	require.NoError(t, os.Remove("nitr.db"))
	present, _ = provisioningSideEffectsPresent()
	assert.False(t, present, "guard should report clean after removing both")
}

// TestServiceSubcommandsRegistered asserts each lifecycle command is wired
// into the root exactly once.
func TestServiceSubcommandsRegistered(t *testing.T) {
	want := []string{"install", "uninstall", "start", "stop", "status"}
	for _, name := range want {
		n := 0
		for _, c := range rootCmd.Commands() {
			if c.Name() == name {
				n++
			}
		}
		assert.Equalf(t, 1, n, "%s registered %d times, want 1", name, n)
	}
}

// TestDoServiceActionMessages covers the result/error mapping of the
// lifecycle core against a fake service, independent of cobra and the OS.
func TestDoServiceActionMessages(t *testing.T) {
	t.Run("install ok", func(t *testing.T) {
		f := &fakeService{}
		msg, err := doServiceAction("install", f)
		require.NoError(t, err)
		assert.Contains(t, msg, "installed via fake-test-platform")
		assert.True(t, f.installed)
	})
	t.Run("install permission denied surfaces hint", func(t *testing.T) {
		f := &fakeService{installErr: errors.New("open /etc/systemd/system/NitrService.service: permission denied")}
		_, err := doServiceAction("install", f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
		assert.Contains(t, err.Error(), "elevated privileges")
	})
	t.Run("uninstall not installed is not an error", func(t *testing.T) {
		f := &fakeService{uninstallErr: service.ErrNotInstalled}
		msg, err := doServiceAction("uninstall", f)
		require.NoError(t, err)
		assert.Contains(t, msg, "not installed")
	})
	t.Run("uninstall other error surfaces", func(t *testing.T) {
		f := &fakeService{uninstallErr: errors.New("boom")}
		_, err := doServiceAction("uninstall", f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})
	t.Run("start not installed suggests install", func(t *testing.T) {
		f := &fakeService{startErr: service.ErrNotInstalled}
		_, err := doServiceAction("start", f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not installed")
		assert.Contains(t, err.Error(), "nitr install")
	})
	t.Run("start ok", func(t *testing.T) {
		f := &fakeService{}
		msg, err := doServiceAction("start", f)
		require.NoError(t, err)
		assert.Contains(t, msg, "started")
		assert.True(t, f.started)
	})
	t.Run("stop ok", func(t *testing.T) {
		f := &fakeService{}
		msg, err := doServiceAction("stop", f)
		require.NoError(t, err)
		assert.Contains(t, msg, "stopped")
		assert.True(t, f.stopped)
	})
	t.Run("status running", func(t *testing.T) {
		f := &fakeService{status: service.StatusRunning}
		msg, err := doServiceAction("status", f)
		require.NoError(t, err)
		assert.Contains(t, msg, "running")
	})
	t.Run("status stopped", func(t *testing.T) {
		f := &fakeService{status: service.StatusStopped}
		msg, err := doServiceAction("status", f)
		require.NoError(t, err)
		assert.Contains(t, msg, "installed but stopped")
	})
	t.Run("status not installed", func(t *testing.T) {
		f := &fakeService{statusErr: service.ErrNotInstalled}
		msg, err := doServiceAction("status", f)
		require.NoError(t, err)
		assert.Contains(t, msg, "not installed")
	})
	t.Run("status unknown error surfaces", func(t *testing.T) {
		f := &fakeService{statusErr: errors.New("systemctl exploded")}
		_, err := doServiceAction("status", f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "systemctl exploded")
	})
}

// TestLifecycleCommandsCreateNoProvisioningFiles is the regression guard for
// the project's recurring bug: CLI commands eagerly initialising config/db so
// that running them creates config.ini and nitr.db in the current directory.
// Each lifecycle command is run through the full cobra dispatch (which is
// where a re-introduced PersistentPreRun or init-time provisioning would
// fire) with a fake service, and must leave the cwd empty. The fake returns
// errors on mutating actions to mirror a no-root host, but side-effect-freeness
// must hold regardless of success/failure.
func TestLifecycleCommandsCreateNoProvisioningFiles(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"install", []string{"install"}},
		{"uninstall", []string{"uninstall"}},
		{"start", []string{"start"}},
		{"stop", []string{"stop"}},
		{"status", []string{"status"}},
		{"install-help", []string{"install", "-h"}},
		{"status-help", []string{"status", "-h"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cdTemp(t)
			// Fake service returns errors for mutating actions (mirrors a
			// non-root host) and StatusUnknown for status; either way the
			// command must exit without provisioning. status on a host where
			// the unit is absent maps to ErrNotInstalled, covered separately.
			fake := &fakeService{
				installErr:   errors.New("permission denied"),
				uninstallErr: errors.New("not loaded"),
				startErr:     errors.New("not loaded"),
				stopErr:      errors.New("not loaded"),
			}
			withFakeService(t, fake, func() {
				_, _ = runRoot(t, tc.args) // errors are tolerated; files are what we check
			})
			assertNoProvisioningSideEffects(t)
		})
	}
}

// TestStatusCommandNotInstalledOnHost drives the REAL kardianos factory on
// the test host (where NitrService is not installed) and asserts the command
// completes without panicking, prints something naming the service, and
// creates no provisioning files. On this host systemd is present and the unit
// is absent, so status resolves to ErrNotInstalled; on a host with no service
// manager at all it resolves to the not-supported error. Both paths must be
// safe and side-effect-free.
func TestStatusCommandNotInstalledOnHost(t *testing.T) {
	cdTemp(t)

	out, err := runRoot(t, []string{"status"})
	// Not installed is a successfully-determined state, not a failure.
	// Unsupported-platform returns a non-nil error, which we also tolerate
	// here; the assertion that matters is no panic + no provisioning files.
	_ = err
	assert.Contains(t, out, ServiceName)
	assertNoProvisioningSideEffects(t)
}

// TestStatusCommandNotInstalledExitCode is the sharper contract: on a host
// where the service is simply not installed, `nitr status` exits 0 and says
// so plainly. It only runs when the real factory can build a service handle
// (i.e. a service manager is detected); otherwise it is skipped, since the
// not-supported path legitimately returns non-zero.
func TestStatusCommandNotInstalledExitCode(t *testing.T) {
	cdTemp(t)
	if _, err := newServiceFunc(); err != nil {
		t.Skipf("no service manager detected on this host: %v", err)
	}

	out, err := runRoot(t, []string{"status"})
	require.NoError(t, err, "status of an absent service must exit 0, not error")
	assert.Contains(t, out, "not installed")
}

// TestUnsupportedPlatformReportsClearly covers requirement #5: when the host
// has no service manager kardianos can drive (service.ErrNoServiceSystemDetected),
// every lifecycle command must report "not supported on this platform" and
// exit non-zero rather than no-op into something that looks like success.
func TestUnsupportedPlatformReportsClearly(t *testing.T) {
	for _, args := range [][]string{
		{"install"}, {"uninstall"}, {"start"}, {"stop"}, {"status"},
	} {
		t.Run(args[0], func(t *testing.T) {
			cdTemp(t)
			orig := newServiceFunc
			newServiceFunc = func() (service.Service, error) {
				return nil, service.ErrNoServiceSystemDetected
			}
			t.Cleanup(func() { newServiceFunc = orig })

			_, err := runRoot(t, args)
			require.Error(t, err, "unsupported platform must exit non-zero, not succeed silently")
			assert.Contains(t, err.Error(), "not supported on this platform")
			assertNoProvisioningSideEffects(t)
		})
	}
}
