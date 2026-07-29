package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"time"

	"github.com/bitcav/go-memdev"
	"github.com/bitcav/nitr-core/bandwidth"
	"github.com/bitcav/nitr-core/baseboard"
	"github.com/bitcav/nitr-core/bios"
	"github.com/bitcav/nitr-core/chassis"
	"github.com/bitcav/nitr-core/cpu"
	"github.com/bitcav/nitr-core/devices"
	"github.com/bitcav/nitr-core/disk"
	"github.com/bitcav/nitr-core/drive"
	"github.com/bitcav/nitr-core/gpu"
	"github.com/bitcav/nitr-core/host"
	"github.com/bitcav/nitr-core/isp"
	"github.com/bitcav/nitr-core/network"
	"github.com/bitcav/nitr-core/overview"
	"github.com/bitcav/nitr-core/product"
	"github.com/bitcav/nitr-core/ram"
	"github.com/bitcav/nitr/handlers"
	"github.com/spf13/cobra"
)

// cliISPTimeout bounds isp.Info()'s outbound, timeout-less HTTP call to
// speedtest.net when run from the CLI. In a server, ISP (handlers/api.go)
// solves this with a cache; a one-shot CLI invocation has nothing to fall
// back to, so it needs its own deadline instead of hanging forever on a
// filtered or air-gapped network.
const cliISPTimeout = 5 * time.Second

// infoResource is one `nitr <name>` command: a collector wrapped uniformly
// as func() (any, error) so the command builder, JSON/table rendering, and
// --watch loop below are all written once and shared by every resource
// instead of per-command.
type infoResource struct {
	name  string
	short string
	fetch func() (any, error)
	// privileged marks resources that need root/Administrator (chassis,
	// baseboard, product, memory): on an empty/failed result, human-readable
	// mode prints a hint instead of a blank table. --json is unaffected --
	// it always prints exactly what the collector returned, matching the
	// API's own behaviour (which does not apply this hint either).
	privileged bool
}

// infoResources is the command surface: subcommand name == API path segment
// (main.go's v1.Get registrations), so the CLI and API share one vocabulary.
var infoResources = []infoResource{
	{"cpu", "Show CPU information", func() (any, error) { return cpu.Info(), nil }, false},
	{"ram", "Show RAM information", func() (any, error) { return ram.Info(), nil }, false},
	{"memory", "Show memory device (DIMM) information", func() (any, error) { return memdev.Info() }, true},
	{"disks", "Show disk usage information", func() (any, error) { return disk.Info(), nil }, false},
	{"drives", "Show physical drive information", func() (any, error) { return drive.Info(), nil }, false},
	{"bios", "Show BIOS information", func() (any, error) { return bios.Info(), nil }, false},
	{"chassis", "Show chassis information", func() (any, error) { return chassis.Info(), nil }, true},
	{"baseboard", "Show baseboard information", func() (any, error) { return baseboard.Info(), nil }, true},
	{"product", "Show product information", func() (any, error) { return product.Info(), nil }, true},
	{"gpu", "Show GPU information", func() (any, error) { return gpu.Info(), nil }, false},
	{"network", "Show network interface information", func() (any, error) { return network.Info(), nil }, false},
	{"bandwidth", "Show network bandwidth usage (samples for ~1s)", func() (any, error) { return bandwidth.Info(), nil }, false},
	{"isp", "Show public IP / ISP information", func() (any, error) { return ispInfoWithTimeout(cliISPTimeout) }, false},
	{"processes", "Show running process information", func() (any, error) { return handlers.Processes() }, false},
	{"devices", "Show PCI device information", func() (any, error) { return devices.Info(), nil }, false},
	{"host", "Show host information", func() (any, error) { return host.Info(), nil }, false},
	{"overview", "Show a summary of host, CPU and RAM", func() (any, error) { return overview.Info(), nil }, false},
}

// ispInfoWithTimeout races isp.Info() (which has no context parameter of
// its own) against timeout. isp.Info() may still be running on its
// goroutine after this returns, the same tradeoff handlers.ISP accepts for
// the same reason.
func ispInfoWithTimeout(timeout time.Duration) (any, error) {
	result := make(chan isp.Setting, 1)
	go func() { result <- isp.Info() }()
	select {
	case s := <-result:
		return s, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("isp lookup timed out after %s (no outbound network access?)", timeout)
	}
}

func buildInfoCmds() []*cobra.Command {
	cmds := make([]*cobra.Command, 0, len(infoResources))
	for _, r := range infoResources {
		cmds = append(cmds, newInfoCmd(r))
	}
	return cmds
}

// newInfoCmd builds the `nitr <r.name>` command. Every collector this
// package calls (nitr-core, go-memdev, handlers.Processes) already returns
// data with no server, no API key, and no network required, matching the
// CEO's brief: "if we want server we run `nitr server`, if we can cli --
// `nitr cpu`".
func newInfoCmd(r infoResource) *cobra.Command {
	cmd := &cobra.Command{
		Use:   r.name,
		Short: r.short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			watch, _ := cmd.Flags().GetString("watch")
			out := cmd.OutOrStdout()

			if watch != "" {
				interval, err := time.ParseDuration(watch)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("invalid --watch interval %q: %w", watch, err)
				}
				runInfoWatch(cmd.Context(), out, r, asJSON, interval)
				return nil
			}

			cmd.SilenceUsage = true
			return runInfoOnce(out, r, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, "print the raw JSON payload (identical to the matching API response)")
	cmd.Flags().StringP("watch", "w", "", "re-fetch and re-render on an interval, e.g. --watch=5s (default 2s with no value)")
	cmd.Flags().Lookup("watch").NoOptDefVal = "2s"
	return cmd
}

// runInfoOnce fetches r once and writes it to w: raw JSON (matching the API
// body exactly -- same encoding/json.Marshal fiber's c.JSON uses, no
// trailing newline) when asJSON, otherwise the human-readable render, with
// the privileged-empty hint substituted in when it applies.
func runInfoOnce(w io.Writer, r infoResource, asJSON bool) error {
	data, err := r.fetch()
	if err != nil {
		if r.privileged && isPermissionDenial(err) {
			return fmt.Errorf("%s: requires elevated privileges -- try `sudo nitr %s` (%w)", r.name, r.name, err)
		}
		return fmt.Errorf("%s: %w", r.name, err)
	}

	if asJSON {
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		_, err = w.Write(raw)
		return err
	}

	if r.privileged && reflect.ValueOf(data).IsZero() {
		_, err := fmt.Fprintf(w, "no %s information available -- requires elevated privileges; try `sudo nitr %s`\n", r.name, r.name)
		return err
	}

	return renderValue(w, data)
}

// runInfoWatch re-renders r on interval until ctx is cancelled (Ctrl+C).
// Per-tick errors are reported to stderr and do not stop the loop -- a
// monitoring loop should keep polling through a transient failure, not exit
// on the first one.
func runInfoWatch(ctx context.Context, w io.Writer, r infoResource, asJSON bool, interval time.Duration) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	for {
		if isTTY(w) {
			_, _ = fmt.Fprint(w, "\x1b[H\x1b[2J")
		}
		if err := runInfoOnce(w, r, asJSON); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		if !asJSON {
			_, _ = fmt.Fprintln(w)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
