package cmd

import (
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var contentFormat string

// textCmd represents the No-Op command.
var textCmd = &cobra.Command{
	Aliases: []string{"t"},
	Short:   "Text provider",
	Use:     "text",
	Example: `  echo "X=1" > configurer l t -- env | grep X`,
	Long: `Text provider will load secrets from text.
Format must be set.

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
Example: echo "X=1" > configurer l t -c "pwd" -c "ls -la" -c "env"

NOTE: Already exported environment variables have precedence over loaded
	  ones. Set the overwrite flag to true to override them.

NOTE: The "-c" flag have precedence over double dash (--)
`,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalln(err)
		}

		// Should be able to override current environment variables.
		shouldOverride := cmd.Flag("override").Value.String() == "true"

		rawValue := cmd.Flag("rawValue").Value.String() == "true"

		dotEnvProvider, err := LoadFromText(shouldOverride, rawValue, contentFormat, string(content))
		if err != nil {
			log.Fatalln(err)
		}

		ConcurrentRunner(dotEnvProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(textCmd)

	loadCmd.Flags().StringVarP(&contentFormat, "format", "f", "env", "env format")

	textCmd.SetUsageTemplate(providerUsageTemplate)
}
