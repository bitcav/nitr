package devices

import (
	"fmt"

	"github.com/jaypipes/ghw"
)

type Device struct {
	Product string `json:"product"`
	Vendor  string `json:"vendor"`
	Address string `json:"address"`
}

func Info() []Device {
	pci, err := ghw.PCI()
	if err != nil {
		fmt.Printf("Error getting PCI info: %v", err)
	}
	devices := pci.ListDevices()
	if len(devices) == 0 {
		fmt.Printf("error: could not retrieve PCI devices\n")
	}

	var devicesArr []Device

	for _, device := range devices {
		vendor := device.Vendor
		vendorName := vendor.Name
		product := device.Product
		productName := product.Name
		devicesArr = append(devicesArr, Device{
			Vendor:  vendorName,
			Product: productName,
			Address: device.Address,
		})
	}

	return devicesArr
}
