package system

import (
	"github.com/bitcav/nitr-agent/baseboard"
	"github.com/bitcav/nitr-agent/bios"
	"github.com/bitcav/nitr-agent/chassis"
	"github.com/bitcav/nitr-agent/cpu"
	"github.com/bitcav/nitr-agent/disk"
	"github.com/bitcav/nitr-agent/drive"
	"github.com/bitcav/nitr-agent/gpu"
	"github.com/bitcav/nitr-agent/host"
	"github.com/bitcav/nitr-agent/network"
	"github.com/bitcav/nitr-agent/process"
	"github.com/bitcav/nitr-agent/product"
	"github.com/bitcav/nitr-agent/ram"
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

func Check() system {
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
