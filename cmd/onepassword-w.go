// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/onepassword"
	"github.com/thalesfsp/configurer/util"
)

// onePasswordWCmd represents the 1Password Connect write command.
var onePasswordWCmd = &cobra.Command{
	Aliases: []string{"op"},
	Short:   "1Password Connect provider",
	Use:     "onepassword",
	Example: "  configurer w --source dev.env onepassword --host https://connect.example.com --token token --vault Production --item Application",
	Long: `1Password Connect provider will upsert fields from a source file into
an item.

The following environment variables can configure the provider:
- OP_CONNECT_HOST: 1Password Connect server URL.
- OP_CONNECT_TOKEN: 1Password Connect token.
- OP_VAULT: Vault name or UUID.
- OP_ITEM: Item title or UUID.`,
	Run: func(cmd *cobra.Command, _ []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		file, err := os.Open(sourceFilename)
		if err != nil {
			log.Fatalln(err)
		}

		defer file.Close()

		values, err := util.ParseFile(ctx, file)
		if err != nil {
			log.Fatalln(err)
		}

		//////
		// Build config.
		//////

		config := &onepassword.Config{
			Host:  cmd.Flag("host").Value.String(),
			Token: cmd.Flag("token").Value.String(),
			Vault: cmd.Flag("vault").Value.String(),
			Item:  cmd.Flag("item").Value.String(),
		}

		onePasswordProvider, err := newOnePasswordProvider(false, false, config)
		if err != nil {
			log.Fatalln(err)
		}

		if err := onePasswordProvider.Write(ctx, values); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(onePasswordWCmd)

	onePasswordWCmd.Flags().String("host", os.Getenv("OP_CONNECT_HOST"), "1Password Connect server URL")
	onePasswordWCmd.Flags().StringP("token", "t", os.Getenv("OP_CONNECT_TOKEN"), "1Password Connect token")
	onePasswordWCmd.Flags().StringP("vault", "v", os.Getenv("OP_VAULT"), "Vault name or UUID")
	onePasswordWCmd.Flags().StringP("item", "i", os.Getenv("OP_ITEM"), "Item title or UUID")

	onePasswordWCmd.SetUsageTemplate(providerUsageTemplate)
}
