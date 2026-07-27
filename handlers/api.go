package handlers

import (
	"fmt"
	"net/http"

	"github.com/bitcav/go-memdev"
	"github.com/bitcav/nitr-core/bandwidth"
	"github.com/bitcav/nitr-core/baseboard"
	"github.com/bitcav/nitr-core/bios"
	"github.com/bitcav/nitr-core/chassis"
	"github.com/bitcav/nitr-core/cpu"
	"github.com/bitcav/nitr-core/devices"
	"github.com/bitcav/nitr-core/disk"
	"github.com/bitcav/nitr-core/drive"
	"github.com/bitcav/nitr-core/gpu"
	"github.com/bitcav/nitr-core/host"
	"github.com/bitcav/nitr-core/isp"
	"github.com/bitcav/nitr-core/network"
	"github.com/bitcav/nitr-core/overview"
	"github.com/bitcav/nitr-core/process"
	"github.com/bitcav/nitr-core/product"
	"github.com/bitcav/nitr-core/ram"
	db "github.com/bitcav/nitr/database"
	"github.com/gofiber/fiber/v2"
)

func AuthAPI(c *fiber.Ctx) error {
	key := c.Get("x-api-key")
	storedKey, err := db.GetApiKey()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	if storedKey == key {
		return c.Next()
	}
	return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
		"message": "Unauthorized, please correct the api key of the target host",
		"status":  http.StatusUnauthorized,
	})
}

// Bandwidth returns a JSON response of the Bandwidth information
func Bandwidth(c *fiber.Ctx) error {
	return c.JSON(bandwidth.Info())
}

// Baseboard returns a JSON response of the Baseboard information
func Baseboard(c *fiber.Ctx) error {
	return c.JSON(baseboard.Info())
}

// Bios returns a JSON response of the Bios information
func Bios(c *fiber.Ctx) error {
	return c.JSON(bios.Info())
}

// Chassis returns a JSON response of the Chassis information
func Chassis(c *fiber.Ctx) error {
	return c.JSON(chassis.Info())
}

// CPU returns a JSON response of the CPUs information
func CPU(c *fiber.Ctx) error {
	return c.JSON(cpu.Info())
}

// Devices returns a JSON response of the Devices information
func Devices(c *fiber.Ctx) error {
	return c.JSON(devices.Info())
}

// Disk returns a JSON response of the Disks information
func Disk(c *fiber.Ctx) error {
	return c.JSON(disk.Info())
}

// Drive returns a JSON response of the Drives information
func Drive(c *fiber.Ctx) error {
	return c.JSON(drive.Info())
}

// GPU returns a JSON response of the GPUs information
func GPU(c *fiber.Ctx) error {
	return c.JSON(gpu.Info())
}

// Host returns a JSON response of the Host information
func Host(c *fiber.Ctx) error {
	return c.JSON(host.Info())
}

// ISP returns a JSON response of the ISP information
func ISP(c *fiber.Ctx) error {
	return c.JSON(isp.Info())
}

// Network returns a JSON response of the Network information
func Network(c *fiber.Ctx) error {
	return c.JSON(network.Info())
}

// Overview returns a JSON response of the Overview information
func Overview(c *fiber.Ctx) error {
	return c.JSON(overview.Info())
}

// Process returns a JSON response of the Processes information
func Process(c *fiber.Ctx) error {
	return c.JSON(process.Info())
}

// Product returns a JSON response of the Product information
func Product(c *fiber.Ctx) error {
	return c.JSON(product.Info())
}

// RAM returns a JSON response of the RAM information
func RAM(c *fiber.Ctx) error {
	return c.JSON(ram.Info())
}

// Memory returns a JSON response of the Memory Devices
func Memory(c *fiber.Ctx) error {
	memInfo, err := memdev.Info()
	if err != nil {
		fmt.Println(err)
	}
	return c.JSON(memInfo)
}
