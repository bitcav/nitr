package disk

import (
	"github.com/gofiber/fiber"
	gdisk "github.com/shirou/gopsutil/disk"
)

type disk struct {
	Mountpoint string  `json:"mountPoint"`
	Free       uint64  `json:"free"`
	Size       uint64  `json:"size"`
	Used       uint64  `json:"used"`
	Percent    float64 `json:"percent"`
}

func checkDisks() []disk {
	disks, _ := gdisk.Partitions(false)

	var totalDisks []disk

	for _, d := range disks {
		diskUsageOf, _ := gdisk.Usage(d.Mountpoint)
		totalDisks = append(totalDisks, disk{
			Free:       diskUsageOf.Free,
			Mountpoint: d.Mountpoint,
			Percent:    diskUsageOf.UsedPercent,
			Size:       diskUsageOf.Total,
			Used:       diskUsageOf.Used,
		})

	}
	return totalDisks
}

func GetDisks(c *fiber.Ctx) {
	c.JSON(checkDisks())
}
