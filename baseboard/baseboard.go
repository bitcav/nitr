package baseboard

import (
	"fmt"

	"github.com/jaypipes/ghw"
)

type BaseBoard struct {
	Vendor       string `json:"vendor"`
	AssetTag     string `json:"assetTag"`
	SerialNumber string `json:"serial"`
	Version      string `json:"version"`
}

func Check() BaseBoard {
	baseboard, err := ghw.Baseboard()
	if err != nil {
		fmt.Printf("Error getting product info: %v", err)
	}
	return BaseBoard{
		Vendor:       baseboard.Vendor,
		AssetTag:     baseboard.AssetTag,
		SerialNumber: baseboard.SerialNumber,
		Version:      baseboard.Version,
	}
}
