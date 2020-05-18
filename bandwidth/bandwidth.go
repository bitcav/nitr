package bandwidth

import "github.com/gofiber/fiber"

func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
