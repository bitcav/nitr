package bandwidth

import (
	"github.com/gofiber/fiber"
)

//Data returns JSON response of the Bandwidth
func Data(c *fiber.Ctx) {
	c.JSON(Check())

}
