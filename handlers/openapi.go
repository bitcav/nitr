package handlers

import (
	"github.com/bitcav/nitr/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/hoisie/mustache"
)

// OpenAPISpecJSON holds the embedded contents of docs/openapi.json, set by
// main.server from its go:embed. Tests substitute a smaller fixture.
var OpenAPISpecJSON []byte

// OpenAPISpec serves the raw OpenAPI 3.1 document that describes every
// /api/v1 route. It is the single source of truth generated docs (Docs,
// below, and README.md's endpoint table) are built from, instead of
// hand-transcribed tables that drift from the code -- see
// main_test.go's TestOpenAPISpecCoversAllRegisteredRoutes for the guard
// that keeps it that way.
//
// Public and unauthenticated, like /health and /ready: a client needs this
// to discover what the x-api-key even protects, so gating it behind that
// same key would be circular.
func OpenAPISpec(c *fiber.Ctx) error {
	c.Type("json")
	return c.Send(OpenAPISpecJSON)
}

// Docs renders the panel's API reference page. The page itself is static;
// it fetches /openapi.json client-side and renders it, so it can never go
// stale independent of the spec the way a server-rendered copy could.
func Docs(c *fiber.Ctx) error {
	docsView, err := view("docs.html")
	if err != nil {
		utils.LogError(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	layoutView, err := view("layout/default.mustache")
	if err != nil {
		utils.LogError(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Type("html")
	return c.SendString(mustache.RenderInLayout(docsView, layoutView))
}
