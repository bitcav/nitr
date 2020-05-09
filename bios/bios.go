package bios

import (
	"fmt"

	"github.com/gofiber/fiber"
	"github.com/jaypipes/ghw"
)

type Bios struct {
	Vendor  string `json:"vendor"`
	Version string `json:"version"`
	Date    string `json:"date"`
}

func Check() Bios {
	bios, err := ghw.BIOS()
	if err != nil {
		fmt.Printf("Error getting BIOS info: %v", err)
	}
	return Bios{
		Vendor:  bios.Vendor,
		Version: bios.Version,
		Date:    bios.Date,
	}
}

func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
