package cmd

import (
	"fmt"

	"github.com/bitcav/nitr/version"
	"github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of nitr",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Nitr v%v", version.Version)
	},
}
