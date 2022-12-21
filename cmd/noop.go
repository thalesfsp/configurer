package cmd

import (
	"context"
	"log"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/noop"
)

// noopCmd represents the No-Op command.
var noopCmd = &cobra.Command{
	Aliases: []string{"n"},
	Short:   "NoOp provider",
	Use:     "noop",
	Example: "  configurer l n -- env | grep PWD",
	Long: `NoOp provider will load secrets from from the
environment and them run the specified command.

About the command to run:
- The <command> to run must be the last argument.
- The command <arguments> must be after the <command>.
- The <command> will inherit the environment variables from the
parent process plus the ones loaded from the provider.

NOTE: A double dash (--) is used to signify the end of command options.
It's required to distinguish between the flags passed to Go and those
that aren't. Everything after the double dash won't be considered to be
Go's flags.`,
	Run: func(cmd *cobra.Command, args []string) {
		command, arguments := splitCmdFromArgs(args)

		// Should be able to override current environment variables.
		shouldOverride := cmd.Flag("override").Value.String() == "true"

		noopProvider, err := noop.New(shouldOverride)
		if err != nil {
			log.Fatalln(err)
		}

		if err := noopProvider.Load(context.Background()); err != nil {
			log.Fatalln(err)
		}

		runCommand(noopProvider, command, arguments)
	},
}

func init() {
	loadCmd.AddCommand(noopCmd)

	noopCmd.SetUsageTemplate(providerUsageTemplate)
}
