package main

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-api/cpu"
	"github.com/juanhuttemann/nitr-api/disk"
	"github.com/juanhuttemann/nitr-api/host"
	"github.com/juanhuttemann/nitr-api/network"
	"github.com/juanhuttemann/nitr-api/process"
	"github.com/juanhuttemann/nitr-api/ram"
)

func setupRoutes(app *fiber.App) {
	app.Get("/api/v1/cpus", cpu.GetCPU)
	app.Get("/api/v1/disks", disk.GetDisks)
	app.Get("/api/v1/host", host.GetHost)
	app.Get("/api/v1/networks", network.GetNetWorks)
	app.Get("/api/v1/processes", process.GetProcess)
	app.Get("/api/v1/ram", ram.GetRAM)
}

func main() {
	app := fiber.New()

	setupRoutes(app)
	app.Listen(3000)
}
