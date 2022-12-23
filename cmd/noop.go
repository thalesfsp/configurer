package cmd

import (
	"context"
	"log"
	"os"

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

## About the command to run

If running only one command:
- The <command> to run must be the last argument.
- The command <arguments> must be after the <command>.
- The <command> will inherit the environment variables from the
parent process plus the ones loaded from the provider.

NOTE: A double dash (--) is used to signify the end of command options.
It's required to distinguish between the flags passed to Go and those
that aren't. Everything after the double dash won't be considered to be
Go's flags.

If running multiple commands:
Use the flag -c to specify the commands to run. The commands must be
comma separated. The commands will be run concurrently.
Example: configurer l n -c "pwd,ls -la,env"

NOTE: Already exported environment variables have precedence over loaded
ones. Set the overwrite flag to true to override them.

NOTE: Double dash (--) have precedence over the "-c" flag.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Should be able to override current environment variables.
		shouldOverride := cmd.Flag("override").Value.String() == "true"

		noopProvider, err := noop.New(shouldOverride)
		if err != nil {
			log.Fatalln(err)
		}

		finalValues, err := noopProvider.Load(context.Background())
		if err != nil {
			log.Fatalln(err)
		}

		// Should be able to dump the loaded values to a file.
		if err := DumpToFile(dumpFilename, finalValues); err != nil {
			log.Fatalln(err)
		}

		errored := false

		for _, exitCode := range ConcurrentRunner(noopProvider, commands, args) {
			if exitCode != 0 {
				errored = true
			}
		}

		if errored {
			os.Exit(1)
		}

		os.Exit(0)
	},
}

func init() {
	loadCmd.AddCommand(noopCmd)

	noopCmd.SetUsageTemplate(providerUsageTemplate)
}
