package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/util"
	"github.com/thalesfsp/configurer/vault"
)

// vaultWCmd represents the vault command.
var vaultWCmd = &cobra.Command{
	Aliases: []string{"v"},
	Short:   "Vault provider",
	Use:     "vault",
	Example: "  configurer w --source dev.env v -a https://v.co -t 123 -m secret -p config",
	Long: `Vault provider will write secrets from Hashicorp Vault

It supports the following authentication methods:
- AppRole
- Token

The following environment variables can be used to configure the provider:
- VAULT_ADDR: The address of the Vault server.
- VAULT_APP_ROLE: The AppRole to use for authentication.
- VAULT_NAMESPACE: The Vault namespace to use for authentication.
- VAULT_TOKEN: The token to use for authentication.

NOTE: If no app role is set, the provider will default to using token.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Context with timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		f, err := os.Open(sourceFilename)
		if err != nil {
			log.Fatalln(err)
		}

		parsedFile, err := util.ParseFile(ctx, f)
		if err != nil {
			log.Fatalln(err)
		}

		auth := &vault.Auth{
			Address:   cmd.Flag("address").Value.String(),
			AppRole:   cmd.Flag("app-role").Value.String(),
			Namespace: cmd.Flag("namespace").Value.String(),
			RoleID:    cmd.Flag("role-id").Value.String(),
			SecretID:  cmd.Flag("secret-id").Value.String(),
			Token:     cmd.Flag("token").Value.String(),
		}

		sI := &vault.SecretInformation{
			MountPath:  cmd.Flag("mount-path").Value.String(),
			SecretPath: cmd.Flag("secret-path").Value.String(),
		}

		vaultProvider, err := vault.New(false, false, auth, sI)
		if err != nil {
			log.Fatalln(err)
		}

		if err := vaultProvider.Write(ctx, parsedFile); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(vaultWCmd)

	// Connection.
	vaultWCmd.Flags().StringP("address", "a", os.Getenv("VAULT_ADDR"), "Address of the Vault server")
	vaultWCmd.Flags().StringP("namespace", "n", os.Getenv("VAULT_NAMESPACE"), "Vault namespace to use for authentication")

	// Path to secret.
	vaultWCmd.Flags().StringP("mount-path", "m", os.Getenv("VAULT_MOUNT_PATH"), "Mount path of the secret")
	vaultWCmd.Flags().StringP("secret-path", "p", os.Getenv("VAULT_SECRET_PATH"), "Path of the secret")

	// Auth.
	vaultWCmd.Flags().StringP("token", "t", os.Getenv("VAULT_TOKEN"), "Token to use for authentication")
	vaultWCmd.Flags().StringP("app-role", "r", os.Getenv("VAULT_APP_ROLE"), "AppRole to use for authentication")
	vaultWCmd.Flags().String("role-id", os.Getenv("VAULT_APP_ROLE_ID"), "AppRole Role ID")
	vaultWCmd.Flags().String("secret-id", os.Getenv("VAULT_APP_SECRET_ID"), "AppRole Secret ID")

	vaultWCmd.SetUsageTemplate(providerUsageTemplate)
}
