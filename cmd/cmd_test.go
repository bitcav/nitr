package cmd

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/bitcav/nitr/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cdTemp(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	dir, err := ioutil.TempDir("", "nitrcmdtest")
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		_ = os.RemoveAll(dir)
	})
	return dir
}

// provisionDefaultUser sets up the database with the default password "123456".
func provisionDefaultUser(t *testing.T) {
	t.Helper()
	require.NoError(t, database.SetupDB())
	require.NoError(t, database.SetUserData("1", models.User{
		Password: utils.PasswordHash("123456"),
		Apikey:   "theapikey",
	}))
}

// withIO replaces os.Stdin with the given input, runs fn, and returns whatever
// was written to os.Stdout during the run.
//
// The stdout pipe is drained concurrently with fn: a command may write far
// more than the OS pipe buffer (~64KB on Linux, ~16KB on macOS) before it
// returns, and reading only after fn would deadlock with no reader attached.
func withIO(t *testing.T, input string, fn func()) string {
	t.Helper()
	tmp, err := os.CreateTemp("", "nitrstdin")
	require.NoError(t, err)
	_, _ = tmp.WriteString(input)
	_, _ = tmp.Seek(0, 0)

	oldIn := os.Stdin
	os.Stdin = tmp
	defer func() { os.Stdin = oldIn; _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldOut := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = oldOut }()

	// Read continuously in the background so writers larger than the pipe
	// buffer never block. The result comes back over a buffered channel so
	// the goroutine can publish even if the test bails out elsewhere.
	type readResult struct {
		b   []byte
		err error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		b, err := io.ReadAll(r)
		resultCh <- readResult{b, err}
	}()

	// Close the writer exactly once on every exit path — including a panic
	// inside fn — so the reader always sees EOF and the goroutine cannot
	// leak. sync.Once keeps the happy path and the deferred close idempotent.
	var closeOnce sync.Once
	closeWriter := func() { closeOnce.Do(func() { _ = w.Close() }) }
	defer closeWriter()

	fn()

	closeWriter()
	res := <-resultCh
	require.NoError(t, res.err)
	return string(res.b)
}

func TestVersionCmdRun(t *testing.T) {
	out := withIO(t, "", func() {
		VersionCmd.Run(VersionCmd, nil)
	})
	assert.Contains(t, out, "Nitr v"+version.Version)
	assert.True(t, strings.HasSuffix(out, "\n"), "version output must end with a newline")
}

func TestExecuteVersion(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	oldArgs := os.Args
	os.Args = []string{"nitr", "version"}
	defer func() { os.Args = oldArgs }()

	out := withIO(t, "", func() { Execute() })
	assert.Contains(t, out, "Nitr v"+version.Version)
	// version is an informational command — it must not provision a database
	// or write a config file. CI and install scripts run "nitr version" as a
	// smoke check, and provisioning a default "123456" user from there is a
	// security hazard in whatever cwd invoked it.
	assertNoProvisioningSideEffects(t)
}

// assertNoProvisioningSideEffects fails if config.ini or nitr.db were created
// in the test's cwd. Used by the side-effect-free command guards below.
func assertNoProvisioningSideEffects(t *testing.T) {
	t.Helper()
	if present, name := provisioningSideEffectsPresent(); present {
		t.Errorf("%s was created; informational commands must be side-effect free", name)
	}
}

// provisioningSideEffectsPresent reports whether config.ini or nitr.db exist
// in the working directory, returning the offending filename. assertNoProvision-
// ingSideEffects wraps it; the helper is kept separate so a test can prove the
// guard is not vacuous by creating the files and confirming this returns true.
func provisioningSideEffectsPresent() (bool, string) {
	if _, err := os.Stat("config.ini"); err == nil {
		return true, "config.ini"
	}
	if _, err := os.Stat("nitr.db"); err == nil {
		return true, "nitr.db"
	}
	return false, ""
}

// TestExecuteVersionHelpClean and TestExecuteRootHelpClean guard the
// regression fixed alongside TestExecuteVersion: cobra dispatches -h, help,
// and unknown commands before any subcommand PreRun runs, so they share the
// same requirement that the cwd stays untouched. All start from genuinely
// empty state — provisionDefaultUser is intentionally NOT called — because
// that helper pre-creates the DB and hides the side effect.
//
// Unknown commands are not covered here: ExecuteArgs calls os.Exit(1) on the
// error cobra returns for them, which terminates the test binary before any
// assertion can run. They go through the same rootCmd.Execute() path as the
// cases below, and that path no longer provisions, so the structural
// guarantee covers them.
func TestExecuteVersionHelpClean(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	withIO(t, "", func() { ExecuteArgs([]string{"-h"}) })
	assertNoProvisioningSideEffects(t)
}

func TestExecuteRootHelpClean(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	withIO(t, "", func() { ExecuteArgs([]string{"help"}) })
	assertNoProvisioningSideEffects(t)
}

func TestExecuteNoDuplicateRegistration(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	oldArgs := os.Args
	os.Args = []string{"nitr", "version"}
	defer func() { os.Args = oldArgs }()

	withIO(t, "", func() { Execute() })
	withIO(t, "", func() { Execute() })

	// cobra lazily injects help/completion during Execute(), so total command
	// count is not stable. Assert each of our subcommands is registered exactly
	// once instead — repeated Execute() must not re-run AddCommand.
	for _, want := range []*cobra.Command{VersionCmd, ApiKey, Passwd, QrCode} {
		n := 0
		for _, c := range rootCmd.Commands() {
			if c == want {
				n++
			}
		}
		assert.Equalf(t, 1, n, "%s registered %d times, want 1", want.Name(), n)
	}
}

func TestApiKeyCorrect(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "123456\n", func() {
		assert.NoError(t, ApiKey.RunE(ApiKey, nil))
	})
	assert.Contains(t, out, "Your api key is: theapikey")
}

func TestApiKeyWrong(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	var runErr error
	withIO(t, "nope\n", func() { runErr = ApiKey.RunE(ApiKey, nil) })
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "Wrong password")
}

func TestPasswdCorrect(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "123456\nnewpass\nnewpass\n", func() {
		assert.NoError(t, Passwd.RunE(Passwd, nil))
	})
	assert.Contains(t, out, "Password changed successfully!")
	u, err := database.GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, utils.PasswordHash("newpass"), u.Password)
}

func TestPasswdWrongCurrent(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	var runErr error
	withIO(t, "bad\n", func() { runErr = Passwd.RunE(Passwd, nil) })
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "Wrong password")
}

func TestPasswdMismatch(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	var runErr error
	withIO(t, "123456\naaa\nbbb\n", func() { runErr = Passwd.RunE(Passwd, nil) })
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "don't match")
	// password must remain unchanged
	u, err := database.GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, utils.PasswordHash("123456"), u.Password)
}

func TestQrCodeCorrect(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "123456\n", func() {
		assert.NoError(t, QrCode.RunE(QrCode, nil))
	})
	// QR output uses block characters; the success path must not print the
	// "wrong password" message and must emit a non-empty payload.
	assert.NotContains(t, out, "Wrong password")
	assert.True(t, len(strings.TrimSpace(out)) > 0)
}

func TestQrCodeWrong(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	var runErr error
	withIO(t, "bad\n", func() { runErr = QrCode.RunE(QrCode, nil) })
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "Wrong password")
}

// TestSubcommandsFailWithoutDB guards the regression at the heart of this
// package: key, passwd, and qr must NOT provision a database when run in a
// directory that doesn't have one. They must fail with a clear, actionable
// error naming the working directory and pointing at the server, and they
// must leave the directory empty. Silently minting an empty credential
// store is how a user ends up with a second database that diverges from the
// one the server actually uses.
//
// It also guards the SilenceUsage-on-this-error-only behaviour in
// requireNitrDB: cobra must print the Error: line and nothing else. A usage
// block would send the user hunting for a syntax mistake that does not
// exist, burying the actionable message.
//
// Uses rootCmd.Execute directly because ExecuteArgs os.Exit(1)s on error,
// which would terminate the test binary before any assertion can run —
// the same reason unknown-command provisioning is not covered here.
func TestSubcommandsFailWithoutDB(t *testing.T) {
	for _, name := range []string{"key", "passwd", "qr"} {
		t.Run(name, func(t *testing.T) {
			cdTemp(t)
			viper.Reset()

			var err error
			stderr := captureStderr(t, func() {
				rootCmd.SetArgs([]string{name})
				err = rootCmd.Execute()
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no nitr database found")
			assert.Contains(t, err.Error(), "start the nitr server")
			// The Error: line is still cobra's, but the usage block that
			// normally follows it must be suppressed — see requireNitrDB.
			assert.Contains(t, stderr, "Error:")
			assert.NotContains(t, stderr, "Usage:")
			assert.NotContains(t, stderr, "Flags:")
			assertNoProvisioningSideEffects(t)
		})
	}
}

// captureStderr runs fn with os.Stderr replaced by an in-memory pipe and
// returns whatever was written. Used to assert on what cobra prints below
// the Error: line. fn must be short: the pipe is drained only after fn
// returns, so writes larger than the OS pipe buffer (~64KB) would deadlock
// — withIO is the tool for large output.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldErr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = oldErr }()

	type readResult struct{ b []byte }
	resultCh := make(chan readResult, 1)
	go func() {
		b, _ := io.ReadAll(r)
		resultCh <- readResult{b}
	}()

	fn()

	require.NoError(t, w.Close())
	return string((<-resultCh).b)
}

// TestWithIODrainsLargeOutput guards the pipe-buffer deadlock fixed in withIO.
// Without a concurrent reader, any command that writes more than the OS pipe
// buffer (~64KB on Linux, ~16KB on macOS) before returning blocks forever.
// The payload here is 4x the larger Linux buffer. The select bounds the
// failure mode so a regression fails the package in seconds instead of
// hanging until the test binary's overall timeout.
func TestWithIODrainsLargeOutput(t *testing.T) {
	const payloadSize = 256 * 1024
	payload := strings.Repeat("x", payloadSize)

	type result struct{ out string }
	done := make(chan result, 1)
	go func() {
		done <- result{out: withIO(t, "", func() {
			_, _ = io.WriteString(os.Stdout, payload)
		})}
	}()

	select {
	case res := <-done:
		assert.Len(t, res.out, payloadSize)
	case <-time.After(10 * time.Second):
		t.Fatal("withIO hung while draining large output; concurrent pipe drain regressed")
	}
}
