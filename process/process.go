package process

import (
	"github.com/gofiber/fiber"
	"github.com/mitchellh/go-ps"
)

type Process struct {
	Pid  int    `json:"pid"`
	Name string `json:"name"`
}

type Processes []Process

func CheckProcesses() []Process {

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

func GetProcess(c *fiber.Ctx) {
	c.JSON(CheckProcesses())
}
