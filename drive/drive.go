package drive

import (
	"fmt"

	"github.com/jaypipes/ghw"
)

//Drive properties
type Drive struct {
	Name      string        `json:"name"`
	DriveType ghw.DriveType `json:"type"`
	Model     string        `json:"model"`
	Serial    string        `json:"serial"`
}

//Check for Drive availability
func Check() []Drive {
	block, err := ghw.Block()
	if err != nil {
		fmt.Printf("Error getting block storage info: %v", err)
	}
	var drvs []Drive
	for _, disk := range block.Disks {
		drvs = append(drvs, Drive{
			Name:      disk.Name,
			DriveType: disk.DriveType,
			Model:     disk.Model,
			Serial:    disk.SerialNumber,
		})
	}

	return drvs
}
