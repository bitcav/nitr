package cmd

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

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
func withIO(t *testing.T, input string, fn func()) string {
	t.Helper()
	tmp, err := ioutil.TempFile("", "nitrstdin")
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

	fn()

	require.NoError(t, w.Close())
	out, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	return string(out)
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
	assert.Contains(t, out, "Password changed succesfully!")
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
