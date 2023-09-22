package cmd

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/option"
)

var (
	commands           []string
	dumpFilename       string
	keyCaserOptions    string
	keyPrefixerOptions string
	keySuffixerOptions string
	shutdownTimeout    time.Duration
)

// loadCmd represents the run command.
var loadCmd = &cobra.Command{
	Aliases: []string{"l"},
	Use:     "load",
	Short:   "Load the configuration from the specified provider",
}

func init() {
	rootCmd.AddCommand(loadCmd)

	loadCmd.PersistentFlags().StringSliceVarP(
		&commands,
		"commands",
		"c",
		[]string{},
		"Set of commands to be executed",
	)

	loadCmd.PersistentFlags().Bool(
		"override",
		false,
		"Override the env var with loaded ones",
	)

	loadCmd.PersistentFlags().StringVarP(
		&dumpFilename,
		"dump",
		"d",
		"",
		"If set, will dump the loaded config to a file. The extension determines the format. Supported are: .env, .json, .yaml | .yml",
	)

	loadCmd.PersistentFlags().DurationVarP(
		&shutdownTimeout,
		"shutdown-timeout",
		"s",
		30*time.Second,
		"The timeout to wait for the command to shutdown",
	)

	loadCmd.PersistentFlags().StringVarP(
		&keyCaserOptions,
		"key-caser",
		"k",
		"",
		"Set the key casing. Supported: "+strings.Join(option.AllowedCases, ","),
	)

	loadCmd.PersistentFlags().StringVarP(
		&keyPrefixerOptions,
		"key-prefixer",
		"x",
		"",
		"Set the key prefix",
	)

	loadCmd.PersistentFlags().StringVar(
		&keyPrefixerOptions,
		"key-suffixer",
		"",
		"Set the key suffix",
	)

	loadCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [provider]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Providers:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [provider] --help" for more information about a provider.{{end}}
`)
}
