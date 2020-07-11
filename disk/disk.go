package disk

import (
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

//Info returns []Disk containing disks information
func Info() []Disk {
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
