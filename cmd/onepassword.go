// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/onepassword"
	"github.com/thalesfsp/configurer/option"
)

var newOnePasswordProvider = onepassword.New

// onePasswordCmd represents the 1Password Connect load command.
var onePasswordCmd = &cobra.Command{
	Aliases: []string{"op"},
	Short:   "1Password Connect provider",
	Use:     "onepassword",
	Example: "  configurer l onepassword --host https://connect.example.com --token token --vault Production --item Application -- env",
	Long: `1Password Connect provider will load fields from an item, export them
to the environment, and then run, if any, the specified command.

The following environment variables can configure the provider:
- OP_CONNECT_HOST: 1Password Connect server URL.
- OP_CONNECT_TOKEN: 1Password Connect token.
- OP_VAULT: Vault name or UUID.
- OP_ITEM: Item title or UUID.

NOTE: Already exported environment variables have precedence over loaded
      ones. Set the override flag to true to override them.`,
	Run: func(cmd *cobra.Command, args []string) {
		shouldOverride := cmd.Flag("override").Value.String() == "true"
		rawValue := cmd.Flag("rawValue").Value.String() == "true"

		//////
		// Build config.
		//////

		config := &onepassword.Config{
			Host:  cmd.Flag("host").Value.String(),
			Token: cmd.Flag("token").Value.String(),
			Vault: cmd.Flag("vault").Value.String(),
			Item:  cmd.Flag("item").Value.String(),
		}

		onePasswordProvider, err := newOnePasswordProvider(shouldOverride, rawValue, config)
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

		finalValues, err := onePasswordProvider.Load(context.Background(), options...)
		if err != nil {
			log.Fatalln(err)
		}

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

		ConcurrentRunner(onePasswordProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(onePasswordCmd)

	onePasswordCmd.Flags().String("host", os.Getenv("OP_CONNECT_HOST"), "1Password Connect server URL")
	onePasswordCmd.Flags().StringP("token", "t", os.Getenv("OP_CONNECT_TOKEN"), "1Password Connect token")
	onePasswordCmd.Flags().StringP("vault", "v", os.Getenv("OP_VAULT"), "Vault name or UUID")
	onePasswordCmd.Flags().StringP("item", "i", os.Getenv("OP_ITEM"), "Item title or UUID")

	onePasswordCmd.SetUsageTemplate(providerUsageTemplate)
}
