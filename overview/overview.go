package overview

import (
	"github.com/bitcav/nitr-agent/cpu"
	"github.com/bitcav/nitr-agent/host"
	"github.com/bitcav/nitr-agent/ram"
	"github.com/gofiber/fiber"
)

type Overview struct {
	Host     host.HostInfo `json:"host"`
	CPUUsage float64       `json:"cpuUsage"`
	RAM      ram.RAM       `json:"ram"`
}

func Check() Overview {
	return Overview{
		Host:     host.Check(),
		CPUUsage: cpu.CpuUsage(),
		RAM:      ram.Check(),
	}
}

func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
