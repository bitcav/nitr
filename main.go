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

func main() {
	app := fiber.New()
	api := app.Group("/api")

	v1 := api.Group("/v1")

	v1.Get("/cpus", cpu.GetCPU)
	v1.Get("/disks", disk.GetDisks)
	v1.Get("/host", host.GetHost)
	v1.Get("/networks", network.GetNetWorks)
	v1.Get("/processes", process.GetProcess)
	v1.Get("/ram", ram.GetRAM)

	app.Listen(3000)
}
