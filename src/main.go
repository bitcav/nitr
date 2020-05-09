package main

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-api/src/cpu"
	"github.com/juanhuttemann/nitr-api/src/ram"
)

func setupRoutes(app *fiber.App) {
	app.Get("/api/v1/cpus", cpu.GetCPU)
	app.Get("/api/v1/ram", cpu.GetRam)
}

func main() {
	app := fiber.New()

	setupRoutes(app)
	app.Listen(3000)
}
