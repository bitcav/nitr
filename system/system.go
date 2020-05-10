package system

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-api/bios"
	"github.com/juanhuttemann/nitr-api/chassis"
	"github.com/juanhuttemann/nitr-api/cpu"
	"github.com/juanhuttemann/nitr-api/disk"
	"github.com/juanhuttemann/nitr-api/drive"
	"github.com/juanhuttemann/nitr-api/gpu"
	"github.com/juanhuttemann/nitr-api/host"
	"github.com/juanhuttemann/nitr-api/network"
	"github.com/juanhuttemann/nitr-api/process"
	"github.com/juanhuttemann/nitr-api/ram"
)

type system struct {
	Host      host.HostInfo          `json:"host"`
	CPU       cpu.CPU                `json:"cpu"`
	Bios      bios.Bios              `json:"bios"`
	RAM       ram.RAM                `json:"ram"`
	Disks     disk.Disks             `json:"disks"`
	Drives    drive.Drives           `json:"drives"`
	Network   network.NetworkDevices `json:"network"`
	GPU       gpu.GPUs               `json:"gpu"`
	Chassis   chassis.Chassis        `json:"chassis"`
	Processes process.Processes      `json:"processes"`
}

func check() system {
	sys := system{
		Host:      host.Check(),
		CPU:       cpu.Check(),
		Bios:      bios.Check(),
		RAM:       ram.Check(),
		Disks:     disk.Check(),
		Drives:    drive.Check(),
		Network:   network.Check(),
		GPU:       gpu.Check(),
		Chassis:   chassis.Check(),
		Processes: process.Check(),
	}
	return sys
}

func Data(c *fiber.Ctx) {
	c.JSON(check())
}
