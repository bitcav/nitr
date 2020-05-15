package overview

import (
	"github.com/gofiber/fiber"
	"github.com/juanhuttemann/nitr-api/cpu"
	"github.com/juanhuttemann/nitr-api/host"
	"github.com/juanhuttemann/nitr-api/ram"
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
