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

		finalValues, err := dotEnvProvider.Load(context.Background())
		if err != nil {
			log.Fatalln(err)
		}

		// Should be able to dump the loaded values to a file.
		if err := DumpToFile(dumpFilename, finalValues); err != nil {
			log.Fatalln(err)
		}

		runCommand(dotEnvProvider, command, arguments)
	},
}

func init() {
	loadCmd.AddCommand(dotEnvCmd)

	dotEnvCmd.Flags().StringSliceVarP(&dotEnvFiles, "files", "f", []string{".env"}, "The dot env files to load.")

	dotEnvCmd.MarkFlagRequired("files")

	dotEnvCmd.SetUsageTemplate(providerUsageTemplate)
}
