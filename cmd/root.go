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
func ExecuteArgs(args []string) {
	//Set Config.ini Default Values
	utils.ConfigFileSetup()

	//Set API Server default Data
	database.SetAPIData()

	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
