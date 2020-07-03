package cpu

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber"
	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/cpu"
)

//CPU properties
type CPU struct {
	Vendor     string    `json:"vendor"`
	Model      string    `json:"model"`
	Cores      uint32    `json:"cores"`
	Threads    uint32    `json:"threads"`
	ClockSpeed float64   `json:"clockSpeed"`
	Usage      float64   `json:"usage"`
	UsageEach  []float64 `json:"usageEach"`
}

//CpuUsage returns the usage percentage of the CPU
func CpuUsage() float64 {
	duration := 500 * time.Millisecond
	cpuUsage, err := cpu.Percent(duration, false)
	if err != nil {
		panic(err)
	}
	return cpuUsage[0]
}

func cpuUsageEach() []float64 {
	duration := 500 * time.Millisecond
	cpuUsage, err := cpu.Percent(duration, true)
	if err != nil {
		panic(err)
	}
	return cpuUsage
}

func cores() uint32 {
	cpu, err := ghw.CPU()
	if err != nil {
		fmt.Printf("Error getting CPU info: %v", err)
	}

	return cpu.TotalCores
}

func threads() uint32 {
	cpu, err := ghw.CPU()
	if err != nil {
		fmt.Printf("Error getting CPU info: %v", err)
	}

	return cpu.TotalThreads
}

func clockSpeed() float64 {
	cpu, err := cpu.Info()
	if err != nil {
		fmt.Printf("Error getting CPU info: %v", err)
	}
	return cpu[0].Mhz
}

func model() string {
	cpu, err := ghw.CPU()
	if err != nil {
		fmt.Printf("Error getting CPU info: %v", err)
	}

	return cpu.Processors[0].Model
}

func vendor() string {
	cpu, err := ghw.CPU()
	if err != nil {
		fmt.Printf("Error getting CPU info: %v", err)
	}

	return cpu.Processors[0].Vendor
}

//Check for CPU availability
func Check() CPU {
	return CPU{
		Vendor:     vendor(),
		Model:      model(),
		Cores:      cores(),
		Threads:    threads(),
		ClockSpeed: clockSpeed(),
		Usage:      CpuUsage(),
		UsageEach:  cpuUsageEach(),
	}
}

//Data returns JSON response of the CPU
func Data(c *fiber.Ctx) {
	c.JSON(Check())
}
