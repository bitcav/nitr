package main

import (
	"fmt"

	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-api/cpu"
	"github.com/juanhuttemann/nitr-api/disk"
	"github.com/juanhuttemann/nitr-api/gpu"
	"github.com/juanhuttemann/nitr-api/host"
	"github.com/juanhuttemann/nitr-api/network"
	"github.com/juanhuttemann/nitr-api/process"
	"github.com/juanhuttemann/nitr-api/ram"
	"github.com/juanhuttemann/nitr-api/system"
)

func main() {
	app := fiber.New(&fiber.Settings{
		DisableStartupMessage: true,
	})

	api := app.Group("/api")

	v1 := api.Group("/v1")

	v1.Get("/", system.Data)
	v1.Get("/cpu", cpu.Data)
	v1.Get("/disks", disk.Data)
	v1.Get("/gpu", gpu.Data)
	v1.Get("/host", host.Data)
	v1.Get("/network", network.Data)
	v1.Get("/processes", process.Data)
	v1.Get("/ram", ram.Data)

	fmt.Println(`                 _  __       
         ____   (_)/ /_ _____
   ____ / __ \ / // __// ___/
 _____ / / / // // /_ / /    
   __ /_/ /_//_/ \__//_/ v0.1.0b     
Running agent on http://localhost:3000/api/v1
`)
	app.Listen(3000)
}
