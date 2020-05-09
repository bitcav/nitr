package network

import "github.com/gofiber/fiber"

type address struct {
	IP string `json:"ip"`
}

type addresses []address

type networkDevice struct {
	Name      string    `json:"name"`
	Addresses addresses `json:"addresses"`
	MAC       string    `json:"mac"`
	Active    bool      `json:"active"`
}

func GetNetWorks(c *fiber.Ctx) {
	c.Send("networks")
}
