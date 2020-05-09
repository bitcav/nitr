package cpu

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber"
	"github.com/shirou/gopsutil/cpu"
)

type CPU struct {
	Brand        string    `json:"brand"`
	Cores        int       `json:"cores"`
	Usage        float64   `json:"usage"`
	UsagePerCore []float64 `json:"usagePerCore"`
}

func cpuUsage() float64 {
	duration := 500 * time.Millisecond
	cpuUsage, err := cpu.Percent(duration, false)
	if err != nil {
		panic(err)
	}
	return cpuUsage[0]
}

func cpuUsagePerCore() []float64 {
	duration := 500 * time.Millisecond
	cpuUsage, err := cpu.Percent(duration, true)
	if err != nil {
		panic(err)
	}
	return cpuUsage
}

func cpuBrand() string {
	var cpuBrand string
	if runtime.GOOS == "windows" {
		out, err := exec.Command("wmic", "cpu", "get", "name").Output()
		if err != nil {
			log.Fatal(err)
		}

		cpuBrand = strings.TrimSpace(strings.Trim(string(out), "Name"))
	} else {
		command := []string{"/proc/cpuinfo"}
		out, err := exec.Command("cat", command...).Output()
		if err != nil {
			fmt.Println("an error has occurred while checking the cpu")
			log.Fatal(err)
		}

		re := regexp.MustCompile(`.*model name.*`)
		matches := re.FindStringSubmatch(string(out))

		cpuBrand = strings.TrimSpace(strings.Trim(strings.Join(matches, " "), "model name"))
		cpuBrand = strings.Trim(cpuBrand, " :")
	}

	return cpuBrand
}

func checkCPU() CPU {
	return CPU{
		Brand:        cpuBrand(),
		Cores:        runtime.NumCPU(),
		Usage:        cpuUsage(),
		UsagePerCore: cpuUsagePerCore(),
	}
}

func Data(c *fiber.Ctx) {
	c.JSON(checkCPU())
}
