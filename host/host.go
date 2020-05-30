package host

import (
	"fmt"
	"runtime"

	"github.com/gofiber/fiber"
	"github.com/shirou/gopsutil/host"
)

//HostInfo properties
type HostInfo struct {
	Name     string `json:"name"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Platform string `json:"platform"`
	Uptime   uint64 `json:"uptime"`
}

//Check for HostInfo availability
func Check() HostInfo {
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

//Data returns JSON response of the Host
func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
