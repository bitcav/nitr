package chassis

import (
	"fmt"

	"github.com/jaypipes/ghw"
)

type Chassis struct {
	ChassisType string `json:"type"`
	Vendor      string `json:"vendor"`
	Serial      string `json:"serial"`
}

func Check() Chassis {
	ghwChassis, err := ghw.Chassis()
	if err != nil {
		fmt.Printf("Error getting chassis info: %v", err)
	}

	return Chassis{
		ChassisType: ghwChassis.TypeDescription,
		Vendor:      ghwChassis.Vendor,
		Serial:      ghwChassis.SerialNumber,
	}
}
