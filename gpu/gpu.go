package gpu

import (
	"fmt"

	"github.com/gofiber/fiber"
	"github.com/jaypipes/ghw"
)

//GPU properties
type GPU struct {
	Brand string `json:"brand"`
	Model string `json:"model"`
}

//Check for GPU availability
func Check() []GPU {
	ghwGpu, err := ghw.GPU()
	if err != nil {
		fmt.Printf("Error getting GPU info: %v", err)
	}

	var gpus []GPU

	for _, card := range ghwGpu.GraphicsCards {
		gpus = append(gpus, GPU{
			Brand: card.DeviceInfo.Vendor.Name,
			Model: card.DeviceInfo.Product.Name,
		})
	}

	return gpus
}

//Handler returns JSON response of the GPUs
func Handler(c *fiber.Ctx) {
	c.JSON(Check())
}
