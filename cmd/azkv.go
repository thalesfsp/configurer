// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/azkv"
	"github.com/thalesfsp/configurer/option"
)

var newAZKVProvider = azkv.New

// azkvCmd represents the Azure Key Vault command.
var azkvCmd = &cobra.Command{
	Aliases: []string{"akv"},
	Short:   "Azure Key Vault secrets provider",
	Use:     "azkv",
	Example: "  configurer l azkv --vault-url https://my-vault.vault.azure.net --secret-name app-secret -- env",
	Long: `Azure Key Vault secrets provider loads secrets from Azure Key Vault,
exports them to the environment, and then runs any specified command.

Authentication uses Azure's default credential chain, including environment
credentials, managed identity, Azure CLI, and Azure Developer CLI.

The following environment variables can be used to configure the provider:
- AZURE_KEY_VAULT_URL: The Azure Key Vault URL.
- AZURE_KEY_VAULT_SECRET_NAMES: Optional comma-separated secret names.

When no secret names are configured, every secret in the vault is listed and
loaded. JSON-object values are exported as separate environment variables.
Plain values use the secret name, with dashes converted to underscores.

NOTE: Already exported environment variables have precedence over loaded
      ones. Set the override flag to true to override them.

NOTE: A double dash (--) marks the end of configurer options. Everything after
      it is treated as the command and arguments to run.`,
	Run: func(cmd *cobra.Command, args []string) {
		shouldOverride := cmd.Flag("override").Value.String() == "true"
		rawValue := cmd.Flag("rawValue").Value.String() == "true"

		secretNames, err := cmd.Flags().GetStringSlice("secret-name")
		if err != nil {
			log.Fatalln(err)
		}

		config := &azkv.Config{
			VaultURL:    cmd.Flag("vault-url").Value.String(),
			SecretNames: secretNames,
		}

		azkvProvider, err := newAZKVProvider(shouldOverride, rawValue, config)
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

		finalValues, err := azkvProvider.Load(context.Background(), options...)
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

		ConcurrentRunner(azkvProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(azkvCmd)

	var secretNames []string
	if value := os.Getenv("AZURE_KEY_VAULT_SECRET_NAMES"); value != "" {
		secretNames = strings.Split(value, ",")
	}

	azkvCmd.Flags().StringP(
		"vault-url",
		"u",
		os.Getenv("AZURE_KEY_VAULT_URL"),
		"Azure Key Vault URL",
	)
	azkvCmd.Flags().StringSliceP(
		"secret-name",
		"s",
		secretNames,
		"Secret names to load (empty lists all secrets)",
	)

	azkvCmd.SetUsageTemplate(providerUsageTemplate)
}
