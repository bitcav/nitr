package main

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-api/src/cpu"
)

func setupRoutes(app *fiber.App) {
	app.Get("/api/v1/cpus", cpu.GetCPU)
	app.Get("/api/v1/ram", ram.GetRam)
}

func main() {
	app := fiber.New()

	setupRoutes(app)
	app.Listen(3000)
}
