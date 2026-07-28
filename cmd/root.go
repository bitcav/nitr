package cmd

import (
	"fmt"
	"os"

	"github.com/bitcav/nitr/utils"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "nitr",
	Short: "Nitr is a remote monitoring tool for system information gathering.",
}

func init() {
	rootCmd.AddCommand(VersionCmd)
	rootCmd.AddCommand(ApiKey)
	rootCmd.AddCommand(Passwd)
	rootCmd.AddCommand(QrCode)
	rootCmd.AddCommand(lifecycleCmds...)
}

func Execute() {
	ExecuteArgs(os.Args[1:])
}

// ExecuteArgs runs the root command with an explicit argv (no program name).
// Exported so main.dispatch can forward the routed args into cobra instead of
// letting cobra fall back to the real os.Args — which is what made the old
// dispatch test a no-op (it parsed the go-test binary's -test.* flags).
//
// It deliberately performs no provisioning of its own: informational
// commands (version, -h, help, unknown) leave the working directory
// untouched, and the credential-using subcommands (key, passwd, qr) refuse
// to run unless nitr.db already exists (see requireNitrDB). main.server is
// the only path that creates nitr.db or config.ini — running "nitr version",
// "nitr -h", or an unknown command must stay side-effect free, since
// "nitr version" is what install scripts and CI smoke tests run, and
// provisioning a default "123456" user from an informational command is not
// acceptable.
func ExecuteArgs(args []string) {
	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// requireNitrDB is the PreRunE for the subcommands that read or write the
// credential store (ApiKey, Passwd, QrCode). It refuses to run unless
// nitr.db already exists in the working directory: silently minting an
// empty credential store is how a user ends up with a second database that
// diverges from the one the server actually uses. main.server remains the
// only thing that creates nitr.db.
//
// Once the database is present, config.ini is loaded into viper so commands
// like `qr` pick up the configured port. config.ini is created by the same
// server setup that creates nitr.db, so loading it here never provisions
// anything that wasn't already there.
func requireNitrDB(cmd *cobra.Command, _ []string) error {
	if _, err := os.Stat("nitr.db"); err != nil {
		// SilenceUsage on this specific error only: the command was typed
		// correctly and printing usage would tell the user to look for a
		// syntax mistake that does not exist, burying the actionable message
		// under boilerplate. Set on the command, not in the struct literal,
		// so genuine usage errors (unknown flag, bad arg) still print usage.
		cmd.SilenceUsage = true
		cwd, _ := os.Getwd()
		return fmt.Errorf("no nitr database found in %s — start the nitr server in this directory first", cwd)
	}
	utils.ConfigFileSetup()
	return nil
}
