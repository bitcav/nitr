package host

import (
	"fmt"
	"runtime"

	"github.com/gofiber/fiber"
	"github.com/shirou/gopsutil/host"
)

type hostInfo struct {
	Name     string `json:"name"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Platform string `json:"platform"`
	Uptime   uint64 `json:"uptime"`
}

func checkHost() hostInfo {

	host, err := host.Info()
	if err != nil {
		fmt.Print(err)
	}

	return hostInfo{
		Name:     host.Hostname,
		OS:       host.OS,
		Arch:     runtime.GOARCH,
		Platform: host.Platform + " " + host.PlatformVersion,
		Uptime:   host.Uptime,
	}
}

func GetHost(c *fiber.Ctx) {
	c.JSON(checkHost())
}
