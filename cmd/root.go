package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	// logOutputs is output used by the SYPL.
	logOutputs []string

	// logSettings is the output's settings.
	logSettings string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "configurer",
	Short: "configurer",
	Long: `Configurer load configuration/secrets from different
sources (providers) and export them as env vars. It allows to run
one or multiple commands with the loaded env vars. See each provider
documentation for more details.`,
}

func init() {
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"default"}, "Available: default, elasticsearch")
	rootCmd.PersistentFlags().StringVar(&logSettings, "log-settings", "", "Log output settings, example (ElasticSearch): `{\"index\": \"configurer\"}`")
}

// Execute adds all child commands to the root command and sets flags
// appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
