package ram

import (
	"fmt"

	"github.com/shirou/gopsutil/mem"
)

//RAM properties
type RAM struct {
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Usage uint64 `json:"usage"`
}

//Info returns RAM struct containing system ram information
func Info() RAM {
	memory, err := mem.VirtualMemory()
	if err != nil {
		fmt.Print(err)
	}
	ram := RAM{
		Free:  memory.Total - memory.Used,
		Usage: memory.Used,
		Total: memory.Total,
	}

	return ram
}
