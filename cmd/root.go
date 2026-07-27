package cmd

import (
	"os"

	"github.com/bitcav/nitr/database"
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
}

func Execute() {
	ExecuteArgs(os.Args[1:])
}

// ExecuteArgs runs the root command with an explicit argv (no program name).
// Exported so main.dispatch can forward the routed args into cobra instead of
// letting cobra fall back to the real os.Args — which is what made the old
// dispatch test a no-op (it parsed the go-test binary's -test.* flags).
//
// It deliberately performs no provisioning of its own: config.ini and nitr.db
// are set up only by the commands that actually touch them (see
// provisionConfigAndDB). Running "nitr version", "nitr -h", or an unknown
// command must stay side-effect free — nitr version is what install scripts
// and CI smoke tests run, and provisioning a default "123456" user from an
// informational command is not acceptable.
func ExecuteArgs(args []string) {
	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// provisionConfigAndDB sets up config.ini and the default API user/database.
// It is wired into the PreRun of the subcommands that read or write either of
// them (ApiKey, Passwd, QrCode) so that informational commands — version, -h,
// unknown — leave the working directory untouched. main.server performs its
// own provisioning, so this helper is only for the CLI subcommands.
func provisionConfigAndDB(*cobra.Command, []string) {
	//Set Config.ini Default Values
	utils.ConfigFileSetup()

	//Set API Server default Data
	database.SetAPIData()
}
