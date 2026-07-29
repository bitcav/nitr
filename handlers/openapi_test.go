package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISpecServesRawBytesWithJSONContentType(t *testing.T) {
	orig := OpenAPISpecJSON
	OpenAPISpecJSON = []byte(`{"openapi":"3.1.0"}`)
	t.Cleanup(func() { OpenAPISpecJSON = orig })

	app := newTestApp()
	app.Get("/openapi.json", OpenAPISpec)

	resp := get(t, app, "/openapi.json")
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	assert.Equal(t, `{"openapi":"3.1.0"}`, body(t, resp))
}

func TestDocsViewRenders(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/docs", Docs)

	resp := get(t, app, "/docs")
	require.Equal(t, 200, resp.StatusCode)
	bd := body(t, resp)
	assert.Contains(t, bd, "docs-endpoints")
	assert.Contains(t, bd, "/openapi.json")
}
