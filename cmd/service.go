package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

// ServiceName is the identity under which nitr registers with the host's
// service manager (systemd, launchd, Windows SCM). It is shared by main,
// which runs the service, and the lifecycle subcommands below, which install
// and control it, so the installer and the runner can never drift apart on
// what they call the service.
const ServiceName = "NitrService"

// ServiceConfig is the single source of truth for the service registration.
// main.initService builds the running service from it; the lifecycle
// commands here build the controller from it. Arguments is left empty on
// purpose: the installed unit must launch the nitr server (no args => main
// runs the host service), not re-run whichever subcommand performed the
// install.
func ServiceConfig() *service.Config {
	return &service.Config{
		Name:        ServiceName,
		DisplayName: "Nitr",
		Description: "A Remote Monitoring Tool for system information gathering, making it available through a JSON API.",
	}
}

// lifecycleProgram is a no-op service.Interface used only when building a
// service handle for install / uninstall / start / stop / status. Those
// commands talk to the OS service manager; they never start the nitr server
// in-process. The real program — the one that runs the fiber server — lives
// in main.go and is what the service manager launches when it starts the
// installed unit. The Interface contract in kardianos v1.2 is Start + Stop
// only (no Init), which is why this matches main.program's method set.
type lifecycleProgram struct{}

func (lifecycleProgram) Start(service.Service) error { return nil }
func (lifecycleProgram) Stop(service.Service) error  { return nil }

// newServiceFunc builds a service handle for lifecycle control. It is a
// package-level var so the lifecycle subcommands can be exercised in tests
// with a fake service instead of driving the real OS service manager.
var newServiceFunc = func() (service.Service, error) {
	return service.New(lifecycleProgram{}, ServiceConfig())
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install nitr as a system service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		svc, err := newServiceFunc()
		if err != nil {
			return unsupported(cmd, err)
		}
		cmd.SilenceUsage = true
		msg, err := doServiceAction("install", svc)
		return finish(cmd, msg, err)
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the nitr system service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		svc, err := newServiceFunc()
		if err != nil {
			return unsupported(cmd, err)
		}
		cmd.SilenceUsage = true
		msg, err := doServiceAction("uninstall", svc)
		return finish(cmd, msg, err)
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the nitr system service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		svc, err := newServiceFunc()
		if err != nil {
			return unsupported(cmd, err)
		}
		cmd.SilenceUsage = true
		msg, err := doServiceAction("start", svc)
		return finish(cmd, msg, err)
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the nitr system service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		svc, err := newServiceFunc()
		if err != nil {
			return unsupported(cmd, err)
		}
		cmd.SilenceUsage = true
		msg, err := doServiceAction("stop", svc)
		return finish(cmd, msg, err)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report whether the nitr service is installed and running on this host",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		svc, err := newServiceFunc()
		if err != nil {
			return unsupported(cmd, err)
		}
		cmd.SilenceUsage = true
		msg, err := doServiceAction("status", svc)
		return finish(cmd, msg, err)
	},
}

// doServiceAction drives a lifecycle action against svc and returns a
// human-readable result line plus an error. It is pure of cmd/os so it can
// be exercised against a fake service in tests. Errors are wrapped with the
// action and, when they read like a permission denial, a privilege hint —
// so "needs root", "worked", and "broken" stay distinguishable.
func doServiceAction(action string, svc service.Service) (string, error) {
	switch action {
	case "install":
		if err := svc.Install(); err != nil {
			return "", explainFail("install", err)
		}
		return fmt.Sprintf("%q installed via %s.", ServiceName, svc.Platform()), nil
	case "uninstall":
		if err := svc.Uninstall(); err != nil {
			if errors.Is(err, service.ErrNotInstalled) {
				return fmt.Sprintf("%q is not installed.", ServiceName), nil
			}
			return "", explainFail("uninstall", err)
		}
		return fmt.Sprintf("%q uninstalled.", ServiceName), nil
	case "start":
		if err := svc.Start(); err != nil {
			if errors.Is(err, service.ErrNotInstalled) {
				return "", fmt.Errorf("%q is not installed; run 'nitr install' first", ServiceName)
			}
			return "", explainFail("start", err)
		}
		return fmt.Sprintf("%q started.", ServiceName), nil
	case "stop":
		if err := svc.Stop(); err != nil {
			if errors.Is(err, service.ErrNotInstalled) {
				return "", fmt.Errorf("%q is not installed; run 'nitr install' first", ServiceName)
			}
			return "", explainFail("stop", err)
		}
		return fmt.Sprintf("%q stopped.", ServiceName), nil
	case "status":
		st, err := svc.Status()
		if errors.Is(err, service.ErrNotInstalled) {
			return fmt.Sprintf("%q is not installed on this host (%s).", ServiceName, svc.Platform()), nil
		}
		if err != nil {
			return "", explainFail("query status of", err)
		}
		switch st {
		case service.StatusRunning:
			return fmt.Sprintf("%q is running.", ServiceName), nil
		case service.StatusStopped:
			return fmt.Sprintf("%q is installed but stopped.", ServiceName), nil
		default:
			return fmt.Sprintf("%q status could not be determined.", ServiceName), nil
		}
	}
	return "", fmt.Errorf("unknown service action %q", action)
}

// finish prints the result line on success, or wraps the error for cobra to
// report and exit non-zero. Centralised so every lifecycle command reports
// the same way.
func finish(cmd *cobra.Command, msg string, err error) error {
	if err != nil {
		return err
	}
	fmt.Println(msg)
	return nil
}

// unsupported reports that the host has no service manager nitr can drive.
// Returns a non-nil error so the process exits non-zero; SilenceUsage is set
// because the command syntax was correct — the platform simply can't do it.
func unsupported(cmd *cobra.Command, err error) error {
	cmd.SilenceUsage = true
	return fmt.Errorf("service control is not supported on this platform: %w", err)
}

// explainFail surfaces a lifecycle failure with the service name and appends
// a privilege hint when the underlying error reads like a permissions
// problem, so "needs root" stays distinguishable from a generic "broken".
func explainFail(action string, err error) error {
	msg := fmt.Sprintf("failed to %s %q: %v", action, ServiceName, err)
	if isPermissionDenial(err) {
		msg += " (this usually requires elevated privileges; try running as root or with sudo)"
	}
	return errors.New(msg)
}

// isPermissionDenial reports whether err looks like an OS permission
// failure, which is how install/start typically present "needs root". The
// string scan covers kardianos's wrapped systemctl / file errors across
// Linux/macOS/Windows; os.ErrPermission covers the stdlib path.
func isPermissionDenial(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "permission denied") ||
		strings.Contains(low, "access denied") ||
		strings.Contains(low, "operation not permitted") ||
		strings.Contains(low, "not permitted")
}
