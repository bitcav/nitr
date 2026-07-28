package cmd

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run the built nitr binary, not RunE in-process, because the
// point of the Run → RunE conversion is the process exit status: a script
// that shells out to `nitr passwd` must be able to tell failure from
// success, and only the real binary proves cobra + ExecuteArgs wire that up.

func buildNitr(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "nitr")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/bitcav/nitr").CombinedOutput()
	require.NoError(t, err, string(out))
	return bin
}

// runNitr executes the binary in dir with the given stdin and returns its
// combined output and exit code. A nil stdin makes the child read from the
// null device, so fmt.Scan hits EOF — the `< /dev/null` case from the ticket.
func runNitr(t *testing.T, bin, dir string, stdin *strings.Reader, args ...string) (string, int) {
	t.Helper()
	c := exec.Command(bin, args...)
	c.Dir = dir
	if stdin != nil {
		c.Stdin = stdin
	}
	out, err := c.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	return string(out), exitErr.ExitCode()
}

// TestCredentialReadErrorExitsNonZero is the ticket's core check:
// `nitr passwd < /dev/null` (and key/qr likewise) must print the read error
// AND exit non-zero, with no usage block dumped after it.
func TestCredentialReadErrorExitsNonZero(t *testing.T) {
	bin := buildNitr(t)
	dir := cdTemp(t)
	provisionDefaultUser(t)

	for _, name := range []string{"passwd", "key", "qr"} {
		t.Run(name, func(t *testing.T) {
			out, code := runNitr(t, bin, dir, nil, name)
			assert.NotEqual(t, 0, code, "%s must exit non-zero on a password read error", name)
			assert.Contains(t, out, "failed to read password")
			assert.NotContains(t, out, "Usage:")
		})
	}
}

// TestPasswdExitCodePolicy pins the policy chosen for the non-read failure
// paths: wrong current password and mismatched new passwords are failed
// invocations too — the command did not do what was asked — so both exit
// non-zero. Success exits 0.
func TestPasswdExitCodePolicy(t *testing.T) {
	bin := buildNitr(t)
	dir := cdTemp(t)
	provisionDefaultUser(t)

	out, code := runNitr(t, bin, dir, strings.NewReader("bad\n"), "passwd")
	assert.NotEqual(t, 0, code, "wrong current password must exit non-zero")
	assert.Contains(t, out, "wrong password")
	assert.NotContains(t, out, "Usage:")

	out, code = runNitr(t, bin, dir, strings.NewReader("123456\naaa\nbbb\n"), "passwd")
	assert.NotEqual(t, 0, code, "mismatched new passwords must exit non-zero")
	assert.Contains(t, out, "don't match")
	assert.NotContains(t, out, "Usage:")

	out, code = runNitr(t, bin, dir, strings.NewReader("123456\nnewpass\nnewpass\n"), "passwd")
	assert.Equal(t, 0, code, "successful password change must exit 0, got %d: %s", code, out)
	assert.Contains(t, out, "Password changed successfully!")
}
