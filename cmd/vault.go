package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/vault"
)

// vaultCmd represents the vault command.
var vaultCmd = &cobra.Command{
	Aliases: []string{"v"},
	Args:    cobra.MinimumNArgs(1),
	Short:   "Vault provider",
	Use:     "vault",
	Example: "  configurer l v -a https://v.co -t 123 -m secret -p config -- env | grep PWD",
	Long: `Vault provider will load secrets from Hashicorp Vault,
exports to the environment, and them run the specified
command.

It supports the following authentication methods:
- AppRole
- Token

The following environment variables can be used to configure the provider:
- VAULT_ADDR: The address of the Vault server.
- VAULT_APP_ROLE: The AppRole to use for authentication.
- VAULT_NAMESPACE: The Vault namespace to use for authentication.
- VAULT_TOKEN: The token to use for authentication.

NOTE: If no app role is set, the provider will default to using token.
NOTE: Alrey exported environment variables have precedence over
loaded ones. Set the overwrite flag to true to override them.

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

		auth := &vault.Auth{
			Address:   cmd.Flag("address").Value.String(),
			AppRole:   cmd.Flag("app-role").Value.String(),
			Namespace: cmd.Flag("namespace").Value.String(),
			Token:     cmd.Flag("token").Value.String(),
		}

		sI := &vault.SecretInformation{
			MountPath:  cmd.Flag("mount-path").Value.String(),
			SecretPath: cmd.Flag("secret-path").Value.String(),
		}

		// Should be able to override current environment variables.
		shouldOverride := cmd.Flag("override").Value.String() == "true"

		vaultProvider, err := vault.New(shouldOverride, auth, sI)
		if err != nil {
			log.Fatalln(err)
		}

		finalValues, err := vaultProvider.Load(context.Background())
		if err != nil {
			log.Fatalln(err)
		}

		// Should be able to dump the loaded values to a file.
		if err := DumpToFile(dumpFilename, finalValues); err != nil {
			log.Fatalln(err)
		}

		runCommand(vaultProvider, command, arguments)
	},
}

func init() {
	loadCmd.AddCommand(vaultCmd)

	vaultCmd.Flags().StringP("address", "a", os.Getenv("VAULT_ADDR"), "Address of the Vault server")
	vaultCmd.Flags().StringP("app-role", "r", os.Getenv("VAULT_APP_ROLE"), "AppRole to use for authentication")
	vaultCmd.Flags().StringP("namespace", "n", os.Getenv("VAULT_NAMESPACE"), "Vault namespace to use for authentication")
	vaultCmd.Flags().StringP("token", "t", os.Getenv("VAULT_TOKEN"), "Token to use for authentication")

	vaultCmd.Flags().StringP("mount-path", "m", "", "Mount path of the secret")
	vaultCmd.Flags().StringP("secret-path", "p", "", "Path of the secret")

	vaultCmd.SetUsageTemplate(providerUsageTemplate)
}
