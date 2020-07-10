package process

import (
	"github.com/gofiber/fiber"
	"github.com/mitchellh/go-ps"
)

//Process properties
type Process struct {
	Pid  int    `json:"pid"`
	Name string `json:"name"`
}

//Check for Processes availability
func Check() []Process {
	processes, err := ps.Processes()
	if err != nil {
		panic(err)
	}
	var processList []Process
	for _, p := range processes {
		proc := Process{Pid: p.Pid(), Name: p.Executable()}
		processList = append(processList, proc)
	}

	return processList
}

//Handler returns JSON response of the Processes
func Handler(c *fiber.Ctx) {
	c.JSON(Check())
}
