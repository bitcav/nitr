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
}

func TestExecuteVersion(t *testing.T) {
	cdTemp(t)
	viper.Reset()

	oldArgs := os.Args
	os.Args = []string{"nitr", "version"}
	defer func() { os.Args = oldArgs }()

	out := withIO(t, "", func() { Execute() })
	assert.Contains(t, out, "Nitr v"+version.Version)
	// Execute provisions config + db on the way through
	assert.FileExists(t, "config.ini")
	assert.FileExists(t, "nitr.db")
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

	out := withIO(t, "123456\n", func() { ApiKey.Run(ApiKey, nil) })
	assert.Contains(t, out, "Your api key is: theapikey")
}

func TestApiKeyWrong(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "nope\n", func() { ApiKey.Run(ApiKey, nil) })
	assert.Contains(t, out, "Wrong password")
}

func TestPasswdCorrect(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "123456\nnewpass\nnewpass\n", func() { Passwd.Run(Passwd, nil) })
	assert.Contains(t, out, "Password changed successfully!")
	u, err := database.GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, utils.PasswordHash("newpass"), u.Password)
}

func TestPasswdWrongCurrent(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "bad\n", func() { Passwd.Run(Passwd, nil) })
	assert.Contains(t, out, "Wrong password")
}

func TestPasswdMismatch(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "123456\naaa\nbbb\n", func() { Passwd.Run(Passwd, nil) })
	assert.Contains(t, out, "don't match")
	// password must remain unchanged
	u, err := database.GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, utils.PasswordHash("123456"), u.Password)
}

func TestQrCodeCorrect(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "123456\n", func() { QrCode.Run(QrCode, nil) })
	// QR output uses block characters; the success path must not print the
	// "wrong password" message and must emit a non-empty payload.
	assert.NotContains(t, out, "Wrong password")
	assert.True(t, len(strings.TrimSpace(out)) > 0)
}

func TestQrCodeWrong(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	provisionDefaultUser(t)

	out := withIO(t, "bad\n", func() { QrCode.Run(QrCode, nil) })
	assert.Contains(t, out, "Wrong password")
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
