package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/version"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeJSON unmarshals a response body into a map. The probe bodies are small
// and flat, so a map is enough without inventing response structs.
func decodeJSON(t *testing.T, resp *http.Response) map[string]string {
	t.Helper()
	var m map[string]string
	require.NoError(t, json.Unmarshal([]byte(body(t, resp)), &m))
	return m
}

// TestHealthOK checks the liveness contract: 200, status "ok", a non-empty
// version matching the build, and that it is reachable with no credentials
// (no x-api-key header, no session cookie).
func TestHealthOK(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/health", Health)

	resp := get(t, app, "/health")
	require.Equal(t, 200, resp.StatusCode)
	assert.NotEqual(t, 302, resp.StatusCode, "must not redirect to login")

	m := decodeJSON(t, resp)
	assert.Equal(t, "ok", m["status"])
	assert.Equal(t, version.Version, m["version"])
	assert.NotEmpty(t, m["version"])
}

// TestHealthNoSideEffects drives /health in a pristine working directory and
// confirms it creates no files. This repo has a history of endpoints spawning
// nitr.db / config.ini unexpectedly; the liveness probe must stay clean.
func TestHealthNoSideEffects(t *testing.T) {
	dir := cdTemp(t)
	app := newTestApp()
	app.Get("/health", Health)

	resp := get(t, app, "/health")
	require.Equal(t, 200, resp.StatusCode)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "/health must not create any files in the working directory")
}

// TestReadyOK reports ready once nitr.db exists (setupEnv creates it).
func TestReadyOK(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/ready", Ready)

	resp := get(t, app, "/ready")
	require.Equal(t, 200, resp.StatusCode)
	assert.NotEqual(t, 302, resp.StatusCode, "must not redirect to login")
	assert.Equal(t, "ready", decodeJSON(t, resp)["status"])
}

// TestReadyNotReady reports 503 when the DB file is absent. It does not
// exercise bolt lock contention (that path is bounded by the stat, never an
// Open), which is the whole point of the narrow probe.
func TestReadyNotReady(t *testing.T) {
	dir := cdTemp(t)
	// Sanity: no DB in the fresh temp dir.
	_, err := os.Stat(filepath.Join(dir, "nitr.db"))
	require.True(t, os.IsNotExist(err))

	app := newTestApp()
	app.Get("/ready", Ready)

	resp := get(t, app, "/ready")
	require.Equal(t, 503, resp.StatusCode)
	assert.NotEqual(t, 302, resp.StatusCode, "must not redirect to login")
	assert.Equal(t, "not ready", decodeJSON(t, resp)["status"])
}

// TestReadyHonorsDataDir is the regression guard for the bug this ticket
// fixes: with --data-dir pointing somewhere other than the cwd, /ready must
// stat nitr.db at database.DBPath(), not the bare "nitr.db" in the cwd.
// The cwd stays empty (no nitr.db anywhere a bare stat could find) and the
// DB is created under data_dir, so a 200 here proves the probe is looking
// in the right place. A stubbed-fs unit test would pass both before and
// after the fix; this one creates the real files.
func TestReadyHonorsDataDir(t *testing.T) {
	dir := cdTemp(t)
	viper.Reset()
	viper.Set("data_dir", filepath.Join(dir, "data"))

	// Sanity: nothing in the cwd, and DBPath resolves under data_dir.
	_, err := os.Stat("nitr.db")
	require.True(t, os.IsNotExist(err), "cwd must be clean so a bare stat would fail")

	// Provision the DB at the resolved location, exactly as the server does
	// at startup (database.SetAPIData -> SetupDB).
	require.NoError(t, db.SetupDB())

	app := newTestApp()
	app.Get("/ready", Ready)

	resp := get(t, app, "/ready")
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "ready", decodeJSON(t, resp)["status"])

	// And the cwd still has no nitr.db — the 200 came from data_dir, not
	// from a bare-stat accidentally finding one.
	_, err = os.Stat("nitr.db")
	assert.True(t, os.IsNotExist(err), "ready must not depend on a nitr.db in the cwd")
}

// TestReadyAbsentUnderDataDir proves the negative: with data_dir set and
// nitr.db genuinely missing from it, /ready must still report 503. A probe
// that cannot fail is worse than none.
func TestReadyAbsentUnderDataDir(t *testing.T) {
	dir := cdTemp(t)
	viper.Reset()
	// Point at a data_dir that exists but has no nitr.db inside.
	dataDir := filepath.Join(dir, "data")
	require.NoError(t, os.Mkdir(dataDir, 0755))
	viper.Set("data_dir", dataDir)

	app := newTestApp()
	app.Get("/ready", Ready)

	resp := get(t, app, "/ready")
	require.Equal(t, 503, resp.StatusCode)
	assert.Equal(t, "not ready", decodeJSON(t, resp)["status"])
}
