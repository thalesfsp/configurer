package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
)

var (
	cliLogger = sypl.NewDefault("cli", level.Info).Breakpoint("ahaha")

	// flushInterval is the default flush interval.
	flushInterval = 1 * time.Second

	// logOutputs is output used by the SYPL.
	logOutputs []string

	// logSettings is the output's settings.
	logSettings string

	// execMode is the execution mode.
	execMode string

	// sequentialDelay is the delay between one command and another.
	sequentialDelay time.Duration
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
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", nil, "Available: default, elasticsearch")
	rootCmd.PersistentFlags().StringVar(&logSettings, "log-settings", "", "Log output settings, example (ElasticSearch): `{\"index\": \"configurer\"}`")

	rootCmd.PersistentFlags().StringVar(&execMode, "exec-mode", "concurrent", "Available: sequential, concurrent")
	rootCmd.PersistentFlags().DurationVar(&sequentialDelay, "sequential-delay", 1*time.Second, "Time between one command and another")

	// flushInterval
	rootCmd.PersistentFlags().DurationVar(&flushInterval, "flush-interval", 1*time.Second, "Flush interval for outputs, if any specified")
}

// Execute adds all child commands to the root command and sets flags
// appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
