package process

import (
	"github.com/gofiber/fiber"
	"github.com/mitchellh/go-ps"
)

type process struct {
	Pid  int    `json:"pid"`
	Name string `json:"name"`
}

type Processes []process

func Check() []process {
	processes, err := ps.Processes()
	if err != nil {
		panic(err)
	}
	var processList []process
	for _, p := range processes {
		proc := process{Pid: p.Pid(), Name: p.Executable()}
		processList = append(processList, proc)
	}

	return processList
}

func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
