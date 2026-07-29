// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/azkv"
	"github.com/thalesfsp/configurer/util"
)

// azkvWCmd represents the Azure Key Vault write command.
var azkvWCmd = &cobra.Command{
	Short:   "Azure Key Vault secrets provider",
	Use:     "azkv",
	Example: "  configurer w --source dev.env azkv --vault-url https://my-vault.vault.azure.net",
	Long: `Azure Key Vault secrets provider writes values to Azure Key Vault.

Authentication uses Azure's default credential chain. Each source key is
stored as a separate secret. Underscores in source keys are converted to
dashes because Azure Key Vault secret names don't support underscores.

The following environment variable can be used to configure the provider:
- AZURE_KEY_VAULT_URL: The Azure Key Vault URL.`,
	Run: func(cmd *cobra.Command, _ []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		file, err := os.Open(sourceFilename)
		if err != nil {
			log.Fatalln(err)
		}

		parsedFile, err := util.ParseFile(ctx, file)
		if err != nil {
			log.Fatalln(err)
		}

		config := &azkv.Config{
			VaultURL: cmd.Flag("vault-url").Value.String(),
		}

		azkvProvider, err := newAZKVProvider(false, false, config)
		if err != nil {
			log.Fatalln(err)
		}

		if err := azkvProvider.Write(ctx, parsedFile); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(azkvWCmd)

	azkvWCmd.Flags().StringP(
		"vault-url",
		"u",
		os.Getenv("AZURE_KEY_VAULT_URL"),
		"Azure Key Vault URL",
	)

	azkvWCmd.SetUsageTemplate(providerUsageTemplate)
}
