package handlers

import (
	"os"

	"github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/version"
	"github.com/gofiber/fiber/v2"
)

// Health is the liveness probe at /health. It is unauthenticated, performs no
// I/O and touches no collector and no bolt handle, so it stays fast and cannot
// fail for reasons unrelated to the process being alive. The build version is
// returned for ops dashboards.
func Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "ok",
		"version": version.Version,
	})
}

// Ready is the readiness probe at /ready. It reports 200 only once nitr.db is
// present and stat-able at its resolved location (database.DBPath — the
// --data-dir flag / NITR_DATA_DIR env / data_dir key, else the working
// directory); 503 otherwise.
//
// It deliberately does NOT call bolt.Open: database.SetupDB opens with nil
// options, and bbolt's exclusive flock with no timeout blocks forever on lock
// contention, so a probe that opened the DB could pile up blocked goroutines
// when contended (an orchestrator polling every few seconds would multiply
// this). A stat is an honest, narrow check: it confirms the DB file exists --
// which only happens once SetAPIData has run, i.e. config setup completed --
// but it does NOT confirm the DB can be opened right now, or that it is not
// corrupt. Upgrading to a true open-and-read probe needs a non-blocking
// bolt.Open, which lives in database/database.go and is owned by ticket
// r66mnwooz52l.
func Ready(c *fiber.Ctx) error {
	if _, err := os.Stat(database.DBPath()); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"error":  err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "ready",
	})
}
