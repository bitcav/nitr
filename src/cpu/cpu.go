package cpu

import (
	"github.com/gofiber/fiber"
)

func GetCPU(c *fiber.Ctx) {
	c.Send("All CPU")
}
