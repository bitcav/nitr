package handlers

import (
	"strings"

	"github.com/bitcav/nitr-core/disk"
	"github.com/bitcav/nitr-core/ram"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	pscpu "github.com/shirou/gopsutil/v4/cpu"
)

// Metrics renders Nitr's system data in Prometheus exposition format at
// /metrics. It is wired behind the same x-api-key auth as /api/v1/* because
// the exposed hardware data is sensitive; Prometheus scrapers pass the header
// via scrape_config (see README).
//
// CPU, RAM and disk are exposed. Bandwidth (bandwidth.Info) is deliberately
// omitted: it sleeps 1s per call (two netdev reads 1s apart) to derive a
// per-interval delta, and a scrape endpoint that blocks for a second causes
// Prometheus scrape timeouts. It will be added once a background sampler
// lands (separate ticket).
func Metrics(c *fiber.Ctx) error {
	registry := prometheus.NewRegistry()
	registry.MustRegister(nitrCollector{})
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return adaptor.HTTPHandler(handler)(c)
}

// nitrCollector exposes CPU, RAM and disk metrics as Prometheus series. It is
// stateless: each scrape re-reads the collectors, so a fresh registry is used
// per request in Metrics to avoid cross-scrape state.
type nitrCollector struct{}

var (
	cpuSecondsDesc = prometheus.NewDesc(
		"nitr_cpu_seconds_total",
		"Cumulative seconds the CPU has spent in each mode, per core. "+
			"Counter; derive utilisation with "+
			"avg(rate(nitr_cpu_seconds_total[5m])) by (mode).",
		[]string{"cpu", "mode"}, nil,
	)
	ramTotalDesc = prometheus.NewDesc(
		"nitr_ram_total_bytes", "Total RAM in bytes.", nil, nil,
	)
	ramFreeDesc = prometheus.NewDesc(
		"nitr_ram_free_bytes", "Free RAM in bytes.", nil, nil,
	)
	ramUsedDesc = prometheus.NewDesc(
		"nitr_ram_used_bytes", "Used RAM in bytes.", nil, nil,
	)
	diskFreeDesc = prometheus.NewDesc(
		"nitr_disk_free_bytes", "Free disk space in bytes.",
		[]string{"mountpoint"}, nil,
	)
	diskSizeDesc = prometheus.NewDesc(
		"nitr_disk_size_bytes", "Total disk size in bytes.",
		[]string{"mountpoint"}, nil,
	)
	diskUsedDesc = prometheus.NewDesc(
		"nitr_disk_used_bytes", "Used disk space in bytes.",
		[]string{"mountpoint"}, nil,
	)
)

func (nitrCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cpuSecondsDesc
	ch <- ramTotalDesc
	ch <- ramFreeDesc
	ch <- ramUsedDesc
	ch <- diskFreeDesc
	ch <- diskSizeDesc
	ch <- diskUsedDesc
}

// cpuModes maps the TimesStat fields we expose to the Prometheus mode label.
// Guest and GuestNice are intentionally absent: the kernel already counts
// them inside user/nice respectively, so emitting them would double-count.
var cpuModes = []struct {
	mode  string
	value func(t pscpu.TimesStat) float64
}{
	{"user", func(t pscpu.TimesStat) float64 { return t.User }},
	{"system", func(t pscpu.TimesStat) float64 { return t.System }},
	{"idle", func(t pscpu.TimesStat) float64 { return t.Idle }},
	{"nice", func(t pscpu.TimesStat) float64 { return t.Nice }},
	{"iowait", func(t pscpu.TimesStat) float64 { return t.Iowait }},
	{"irq", func(t pscpu.TimesStat) float64 { return t.Irq }},
	{"softirq", func(t pscpu.TimesStat) float64 { return t.Softirq }},
	{"steal", func(t pscpu.TimesStat) float64 { return t.Steal }},
}

func (nitrCollector) Collect(ch chan<- prometheus.Metric) {
	collectCPU(ch)
	collectRAM(ch)
	collectDisk(ch)
}

// collectCPU reads cumulative CPU time counters straight from the kernel via
// gopsutil. Times() does not sleep (unlike nitr-core's cpu.Info, which calls
// cpu.Percent(500ms) twice). On error the CPU series is skipped for this
// scrape rather than failing the whole endpoint: a partial scrape is much
// more useful to Prometheus than a dead one.
func collectCPU(ch chan<- prometheus.Metric) {
	times, err := pscpu.Times(true)
	if err != nil {
		return
	}
	for _, t := range times {
		// gopsutil reports per-core names like "cpu0"; strip the prefix so
		// the cpu label matches the node_exporter convention ("0").
		cpu := strings.TrimPrefix(t.CPU, "cpu")
		for _, m := range cpuModes {
			ch <- prometheus.MustNewConstMetric(
				cpuSecondsDesc, prometheus.CounterValue, m.value(t), cpu, m.mode,
			)
		}
	}
}

func collectRAM(ch chan<- prometheus.Metric) {
	r := ram.Info()
	ch <- prometheus.MustNewConstMetric(ramTotalDesc, prometheus.GaugeValue, float64(r.Total))
	ch <- prometheus.MustNewConstMetric(ramFreeDesc, prometheus.GaugeValue, float64(r.Free))
	ch <- prometheus.MustNewConstMetric(ramUsedDesc, prometheus.GaugeValue, float64(r.Usage))
}

func collectDisk(ch chan<- prometheus.Metric) {
	for _, d := range disk.Info() {
		ch <- prometheus.MustNewConstMetric(diskFreeDesc, prometheus.GaugeValue, float64(d.Free), d.Mountpoint)
		ch <- prometheus.MustNewConstMetric(diskSizeDesc, prometheus.GaugeValue, float64(d.Size), d.Mountpoint)
		ch <- prometheus.MustNewConstMetric(diskUsedDesc, prometheus.GaugeValue, float64(d.Used), d.Mountpoint)
	}
}
