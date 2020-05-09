package gpu

import (
	"fmt"

	"github.com/gofiber/fiber"
	"github.com/jaypipes/ghw"
)

type gpu struct {
	Brand string `json:"brand"`
	Model string `json:"model"`
}

type GPUs []gpu

func Check() GPUs {
	ghwGpu, err := ghw.GPU()
	if err != nil {
		fmt.Printf("Error getting GPU info: %v", err)
	}

	var gpus GPUs

	for _, card := range ghwGpu.GraphicsCards {
		gpus = append(gpus, gpu{
			Brand: card.DeviceInfo.Vendor.Name,
			Model: card.DeviceInfo.Product.Name,
		})
	}

	return gpus
}

func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
