package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitcav/nitr/version"
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
