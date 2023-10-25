package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/noop"
	"github.com/thalesfsp/configurer/option"
)

// noopCmd represents the No-Op command.
var noopCmd = &cobra.Command{
	Aliases: []string{"n"},
	Short:   "NoOp provider",
	Use:     "noop",
	Example: "  configurer l n -- env | grep PWD",
	Long: `NoOp provider will load secrets from the
environment, and them run if any, the specified command.

## About the command to run

If running only one command:
- The <command> to run must be the last argument.
- The command <arguments> must be after the <command>.
- The <command> will inherit the environment variables from the
parent process plus the ones loaded from the provider.

NOTE: A double dash (--) is used to signify the end of command options.
      It's required to distinguish between the flags passed to Go and
	  those that aren't. Everything after the double dash won't be
	  considered to be Go's flags.

If running multiple commands:
Use as many -c flags you want, to specify the commands to run.
The commands will be run concurrently.
Example: configurer l n -c "pwd" -c "ls -la" -c "env"

NOTE: Already exported environment variables have precedence over loaded
	  ones. Set the overwrite flag to true to override them.

NOTE: The "-c" flag have precedence over double dash (--)
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Should be able to override current environment variables.
		shouldOverride := cmd.Flag("override").Value.String() == "true"

		rawValue := cmd.Flag("rawValue").Value.String() == "true"

		noopProvider, err := noop.New(shouldOverride, rawValue)
		if err != nil {
			log.Fatalln(err)
		}

		var options []option.LoadKeyFunc

		if keyCaserOptions != "" {
			options = append(options, option.WithKeyCaser(keyCaserOptions))
		}

		if keyPrefixerOptions != "" {
			options = append(options, option.WithKeyPrefixer(keyPrefixerOptions))
		}

		if keySuffixerOptions != "" {
			options = append(options, option.WithKeySuffixer(keySuffixerOptions))
		}

		finalValues, err := noopProvider.Load(context.Background(), options...)
		if err != nil {
			log.Fatalln(err)
		}

		// Should be able to dump the loaded values to a file.
		if dumpFilename != "" {
			file, err := os.Create(dumpFilename)
			if err != nil {
				log.Fatalln(err)
			}

			defer file.Close()

			if err := DumpToFile(file, finalValues, rawValue); err != nil {
				log.Fatalln(err)
			}
		}

		ConcurrentRunner(noopProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(noopCmd)

	noopCmd.SetUsageTemplate(providerUsageTemplate)
}
