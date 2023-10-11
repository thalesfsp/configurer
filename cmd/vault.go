package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/vault"
)

// vaultCmd represents the vault command.
var vaultCmd = &cobra.Command{
	Aliases: []string{"v"},
	Short:   "Vault provider",
	Use:     "vault",
	Example: "  configurer l v -a https://v.co -t 123 -m secret -p config -- env | grep PWD",
	Long: `Vault provider will load secrets from Hashicorp Vault,
exports to the environment, and them run if any, the specified
command.

It supports the following authentication methods:
- AppRole
- Token

The following environment variables can be used to configure the provider:
- VAULT_ADDR: The address of the Vault server.
- VAULT_APP_ROLE_ID: AppRole Role ID
- VAULT_APP_ROLE: The AppRole to use for authentication.
- VAULT_APP_SECRET_ID: AppRole Secret ID
- VAULT_NAMESPACE: The Vault namespace to use for authentication.
- VAULT_TOKEN: The token to use for authentication.

NOTE: If no app role is set, the provider will default to using token.

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
Example: configurer l v -a https://v.co -t 123 -m secret -p config -c "ls -la" -c "env"

NOTE: Already exported environment variables have precedence over loaded
      ones. Set the overwrite flag to true to override them.

NOTE: The "-c" flag have precedence over double dash (--)

## Setting up the environment to run the application

There are two methods to set up the environment to run the application.

### Flags (not recommended)

Values are set by specifying flags. In the following example, values are
set and then the env command is run.

  configurer l v \
      --address     "{address}" \
      --role-id     "xyz" \
      --app-role    "{project_name}" \
      --secret-id   "xyz" \
      --mount-path  "kv" \
      --namespace   "{namespace}" \
      --secret-path "/{project_name}/{environment}/{service_name}/main" -- env

### Environment Variables (this is the recommended, and preferred way)

Setup values are set by specifying environment variables. In the following
example, values are set and then the env command is run. It's cleaner and
more secure.

  export VAULT_ADDR="{address}"
  export VAULT_APP_ROLE_ID="xyz"
  export VAULT_APP_ROLE={project_name}
  export VAULT_APP_SECRET_ID="xyz"
  export VAULT_MOUNT_PATH="kv"
  export VAULT_NAMESPACE="{namespace}"
  export VAULT_SECRET_PATH="/{project_name}/{environment}/{service_name}/main"

  configurer l v -- env`,
	Run: func(cmd *cobra.Command, args []string) {
		// Should be able to override current environment variables.
		shouldOverride := cmd.Flag("override").Value.String() == "true"

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

		vaultProvider, err := vault.New(shouldOverride, auth, sI)
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

		finalValues, err := vaultProvider.Load(context.Background(), options...)
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

			if err := DumpToFile(file, finalValues); err != nil {
				log.Fatalln(err)
			}
		}

		ConcurrentRunner(vaultProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(vaultCmd)

	// Connection.
	vaultCmd.Flags().StringP("address", "a", os.Getenv("VAULT_ADDR"), "Address of the Vault server")
	vaultCmd.Flags().StringP("namespace", "n", os.Getenv("VAULT_NAMESPACE"), "Vault namespace to use for authentication")

	// Path to secret.
	vaultCmd.Flags().StringP("mount-path", "m", os.Getenv("VAULT_MOUNT_PATH"), "Mount path of the secret")
	vaultCmd.Flags().StringP("secret-path", "p", os.Getenv("VAULT_SECRET_PATH"), "Path of the secret")

	// Auth.
	vaultCmd.Flags().StringP("token", "t", os.Getenv("VAULT_TOKEN"), "Token to use for authentication")
	vaultCmd.Flags().StringP("app-role", "r", os.Getenv("VAULT_APP_ROLE"), "AppRole to use for authentication")
	vaultCmd.Flags().String("role-id", os.Getenv("VAULT_APP_ROLE_ID"), "AppRole Role ID")
	vaultCmd.Flags().String("secret-id", os.Getenv("VAULT_APP_SECRET_ID"), "AppRole Secret ID")

	vaultCmd.SetUsageTemplate(providerUsageTemplate)
}
