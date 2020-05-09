package main

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-api/cpu"
	"github.com/juanhuttemann/nitr-api/process"
	"github.com/juanhuttemann/nitr-api/ram"
)

func setupRoutes(app *fiber.App) {
	app.Get("/api/v1/cpus", cpu.GetCPU)
	app.Get("/api/v1/ram", ram.GetRAM)
	app.Get("/api/v1/processes", process.GetProcess)

}

func main() {
	app := fiber.New()

	setupRoutes(app)
	app.Listen(3000)
}
