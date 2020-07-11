package host

import (
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/host"
)

//HostInfo properties
type HostInfo struct {
	Name     string `json:"name"`
	OS       string `json:"os"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Uptime   uint64 `json:"uptime"`
}

//Info returns HostInfo struct containing host information
func Info() HostInfo {
	host, err := host.Info()
	if err != nil {
		fmt.Print(err)
	}

	return HostInfo{
		Name:     host.Hostname,
		OS:       host.OS,
		Arch:     runtime.GOARCH,
		Platform: host.Platform + " " + host.PlatformVersion,
		Uptime:   host.Uptime,
	}
}
