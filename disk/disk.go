package disk

import (
	"github.com/gofiber/fiber"
	gdisk "github.com/shirou/gopsutil/disk"
)

//Disk properties
type Disk struct {
	Mountpoint string  `json:"mountPoint"`
	Free       uint64  `json:"free"`
	Size       uint64  `json:"size"`
	Used       uint64  `json:"used"`
	Percent    float64 `json:"percent"`
}

//Check for Disks availability
func Check() []Disk {
	disks, _ := gdisk.Partitions(false)
	var totalDisks []Disk

	for _, d := range disks {
		diskUsageOf, _ := gdisk.Usage(d.Mountpoint)
		if d.Fstype != "squashfs" {
			totalDisks = append(totalDisks, Disk{
				Free:       diskUsageOf.Free,
				Mountpoint: d.Mountpoint,
				Percent:    diskUsageOf.UsedPercent,
				Size:       diskUsageOf.Total,
				Used:       diskUsageOf.Used,
			})
		}

	}
	return totalDisks
}

//Handler returns JSON response of the Disks
func Handler(c *fiber.Ctx) {
	c.JSON(Check())
}
