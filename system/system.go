package system

import (
	"github.com/bitcav/nitr/baseboard"
	"github.com/bitcav/nitr/bios"
	"github.com/bitcav/nitr/chassis"
	"github.com/bitcav/nitr/cpu"
	"github.com/bitcav/nitr/disk"
	"github.com/bitcav/nitr/drive"
	"github.com/bitcav/nitr/gpu"
	"github.com/bitcav/nitr/host"
	"github.com/bitcav/nitr/network"
	"github.com/bitcav/nitr/process"
	"github.com/bitcav/nitr/product"
	"github.com/bitcav/nitr/ram"
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

func Info() system {
	sys := system{
		Host:      host.Info(),
		CPU:       cpu.Info(),
		Bios:      bios.Info(),
		RAM:       ram.Info(),
		Disks:     disk.Info(),
		Drives:    drive.Info(),
		Network:   network.Info(),
		GPU:       gpu.Info(),
		Chassis:   chassis.Info(),
		Processes: process.Info(),
		BaseBoard: baseboard.Info(),
		Product:   product.Info(),
	}
	return sys
}
