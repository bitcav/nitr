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
	//Set Config.ini Default Values
	utils.ConfigFileSetup()

	//Set API Server default Data
	database.SetAPIData()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
