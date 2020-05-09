package system

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-api/cpu"
	"github.com/juanhuttemann/nitr-api/disk"
	"github.com/juanhuttemann/nitr-api/host"
	"github.com/juanhuttemann/nitr-api/network"
	"github.com/juanhuttemann/nitr-api/process"
	"github.com/juanhuttemann/nitr-api/ram"
)

type system struct {
	Host      host.HostInfo          `json:"host"`
	CPU       cpu.CPU                `json:"cpu"`
	RAM       ram.RAM                `json:"ram"`
	Disks     disk.Disks             `json:"disks"`
	Network   network.NetworkDevices `json:"network"`
	Processes process.Processes      `json:"processes"`
}

func check() system {
	return system{
		Host:      host.Check(),
		CPU:       cpu.Check(),
		RAM:       ram.Check(),
		Disks:     disk.Check(),
		Network:   network.Check(),
		Processes: process.Check(),
	}
}

func Data(c *fiber.Ctx) {
	c.JSON(check())
}
