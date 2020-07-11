package bios

import (
	"fmt"

	"github.com/jaypipes/ghw"
)

//Bios properties
type Bios struct {
	Vendor  string `json:"vendor"`
	Version string `json:"version"`
	Date    string `json:"date"`
}

//Info returns Bios struct containing motherboad bios information
func Info() Bios {
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
