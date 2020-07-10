package bandwidth

import (
	"github.com/gofiber/fiber"
)

//Handler returns JSON response of the Bandwidth
func Handler(c *fiber.Ctx) {
	c.JSON(Check())

}
