package bandwidth

import (
	"fmt"

	"github.com/gofiber/fiber"
	"github.com/kylegrantlucas/speedtest"
)

type Bandwidth struct {
	Ping     float64 `json:"ping"`
	Download float64 `json:"download"`
	Upload   float64 `json:"upload"`
}

func Check() Bandwidth {
	client, err := speedtest.NewDefaultClient()
	if err != nil {
		fmt.Printf("error creating client: %v", err)
	}

	// Pass an empty string to select the fastest server
	server, err := client.GetServer("")
	if err != nil {
		fmt.Printf("error getting server: %v", err)
	}

	dmbpsChan := make(chan float64)
	umbpsChan := make(chan float64)

	go func(c chan float64) {
		dmbps, err := client.Download(server)
		if err != nil {
			fmt.Printf("error getting download: %v", err)
		}
		c <- dmbps
	}(dmbpsChan)

	go func(c chan float64) {
		umbps, err := client.Upload(server)
		if err != nil {
			fmt.Printf("error getting upload: %v", err)
		}
		c <- umbps
	}(umbpsChan)

	return Bandwidth{
		Ping:     server.Latency,
		Download: <-dmbpsChan,
		Upload:   <-umbpsChan,
	}
}

func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
