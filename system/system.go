package system

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-agent/baseboard"
	"github.com/juanhuttemann/nitr-agent/bios"
	"github.com/juanhuttemann/nitr-agent/chassis"
	"github.com/juanhuttemann/nitr-agent/cpu"
	"github.com/juanhuttemann/nitr-agent/disk"
	"github.com/juanhuttemann/nitr-agent/drive"
	"github.com/juanhuttemann/nitr-agent/gpu"
	"github.com/juanhuttemann/nitr-agent/host"
	"github.com/juanhuttemann/nitr-agent/network"
	"github.com/juanhuttemann/nitr-agent/process"
	"github.com/juanhuttemann/nitr-agent/product"
	"github.com/juanhuttemann/nitr-agent/ram"
)

type system struct {
	Host      host.HostInfo          `json:"host"`
	CPU       cpu.CPU                `json:"cpu"`
	Bios      bios.Bios              `json:"bios"`
	RAM       ram.RAM                `json:"ram"`
	Disks     []disk.Disk            `json:"disks"`
	Drives    []drive.Drive          `json:"drives"`
	Network   network.NetworkDevices `json:"network"`
	GPU       []gpu.GPU              `json:"gpu"`
	BaseBoard baseboard.BaseBoard    `json:"baseboard"`
	Product   product.Product        `json:"product"`
	Chassis   chassis.Chassis        `json:"chassis"`
	Processes []process.Process      `json:"processes"`
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
		BaseBoard: baseboard.Check(),
		Product:   product.Check(),
	}
	return sys
}

//Data returns JSON response of the entire System
func Data(c *fiber.Ctx) {
	c.JSON(check())
}
