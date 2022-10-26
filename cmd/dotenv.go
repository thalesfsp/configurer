package cmd

import (
	"context"
	"log"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/dotenv"
)

var dotEnvFiles []string

// dotEnvCmd represents the vault command.
var dotEnvCmd = &cobra.Command{
	Aliases: []string{"d"},
	Args:    cobra.MinimumNArgs(1),
	Short:   "DotEnv provider",
	Use:     "dotenv",
	Example: "  configurer l d -f .env -- env | grep PWD",
	Long: `DotEnv provider will load secrets from .env files,
exports to the environment, and them run the specified
command.

About the command to run:
- The <command> to run must be the last argument.
- The command <arguments> must be after the <command>.
- The <command> will inherit the environment variables from the
parent process plus the ones loaded from the provider.

NOTE: Alrey exported environment variables have precedence over
loaded ones. Set the overwrite flag to true to override them.
NOTE: A double dash (--) is used to signify the end of command options.
It's required to distinguish between the flags passed to Go and those
that aren't. Everything after the double dash won't be considered to be
Go's flags.`,
	Run: func(cmd *cobra.Command, args []string) {
		command, arguments := splitCmdFromArgs(args)

		// Should be able to override current environment variables.
		shouldOverride := cmd.Flag("override").Value.String() == "true"

		dotEnvProvider, err := dotenv.New(shouldOverride, dotEnvFiles...)
		if err != nil {
			log.Fatalln(err)
		}

		if err := dotEnvProvider.Load(context.Background()); err != nil {
			log.Fatalln(err)
		}

		runCommand(dotEnvProvider, command, arguments)
	},
}

func init() {
	loadCmd.AddCommand(dotEnvCmd)

	dotEnvCmd.Flags().StringSliceVarP(&dotEnvFiles, "files", "f", []string{".env"}, "The dot env files to load.")

	dotEnvCmd.MarkFlagRequired("files")

	dotEnvCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}} -- [command to run] [command args]{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
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

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)
}
