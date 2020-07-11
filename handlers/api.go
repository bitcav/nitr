package handlers

import (
	"github.com/bitcav/nitr/bandwidth"
	"github.com/bitcav/nitr/baseboard"
	"github.com/bitcav/nitr/bios"
	"github.com/bitcav/nitr/chassis"
	"github.com/bitcav/nitr/cpu"
	"github.com/bitcav/nitr/devices"
	"github.com/bitcav/nitr/disk"
	"github.com/bitcav/nitr/drive"
	"github.com/bitcav/nitr/gpu"
	"github.com/bitcav/nitr/host"
	"github.com/bitcav/nitr/isp"
	"github.com/bitcav/nitr/network"
	"github.com/bitcav/nitr/overview"
	"github.com/bitcav/nitr/process"
	"github.com/bitcav/nitr/product"
	"github.com/bitcav/nitr/ram"
	"github.com/gofiber/fiber"
)

//Bandwidth returns a JSON response of the Bandwidth information
func Bandwidth(c *fiber.Ctx) {
	c.JSON(bandwidth.Check())
}

//Baseboard returns a JSON response of the Baseboard information
func Baseboard(c *fiber.Ctx) {
	c.JSON(baseboard.Check())
}

//Bios returns a JSON response of the Bios information
func Bios(c *fiber.Ctx) {
	c.JSON(bios.Check())
}

//Chassis returns a JSON response of the Chassis information
func Chassis(c *fiber.Ctx) {
	c.JSON(chassis.Check())
}

//CPU returns a JSON response of the CPUs information
func CPU(c *fiber.Ctx) {
	c.JSON(cpu.Check())
}

//Devices returns a JSON response of the Devices information
func Devices(c *fiber.Ctx) {
	c.JSON(devices.Check())
}

//Disk returns a JSON response of the Disks information
func Disk(c *fiber.Ctx) {
	c.JSON(disk.Check())
}

//Drive returns a JSON response of the Drives information
func Drive(c *fiber.Ctx) {
	c.JSON(drive.Check())
}

//GPU returns a JSON response of the GPUs information
func GPU(c *fiber.Ctx) {
	c.JSON(gpu.Check())
}

//Host returns a JSON response of the Host information
func Host(c *fiber.Ctx) {
	c.JSON(host.Check())
}

//ISP returns a JSON response of the ISP information
func ISP(c *fiber.Ctx) {
	c.JSON(isp.Check())
}

//Network returns a JSON response of the Network information
func Network(c *fiber.Ctx) {
	c.JSON(network.Check())
}

//Overview returns a JSON response of the Overview information
func Overview(c *fiber.Ctx) {
	c.JSON(overview.Check())
}

//Process returns a JSON response of the Processes information
func Process(c *fiber.Ctx) {
	c.JSON(process.Check())
}

//Product returns a JSON response of the Product information
func Product(c *fiber.Ctx) {
	c.JSON(product.Check())
}

//RAM returns a JSON response of the RAM information
func RAM(c *fiber.Ctx) {
	c.JSON(ram.Check())
}
