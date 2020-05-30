package bios

import (
	"fmt"

	"github.com/gofiber/fiber"
	"github.com/jaypipes/ghw"
)

//Bios properties
type Bios struct {
	Vendor  string `json:"vendor"`
	Version string `json:"version"`
	Date    string `json:"date"`
}

//Check for Bios availability
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

//Data returns JSON response of the Bios
func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
