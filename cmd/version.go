package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/internal/version"
)

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(version.Get())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
