package overview

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-agent/cpu"
	"github.com/juanhuttemann/nitr-agent/host"
	"github.com/juanhuttemann/nitr-agent/ram"
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
