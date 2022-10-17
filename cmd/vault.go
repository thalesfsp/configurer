package cmd

import (
	"context"
	"log"
	"os"
	"os/exec"

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
		var (
			command   string
			arguments []string
		)

		for i := 0; i < len(args); i++ {
			// If the first argument is the command, set command, else set arguments.
			if i == 0 {
				command = args[i]
			} else {
				arguments = append(arguments, args[i])
			}
		}

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

		shouldOverride := cmd.Flag("override").Value.String() == "true"

		vProvider, err := vault.New(shouldOverride, auth, sI)
		if err != nil {
			log.Fatalln(err)
		}

		if err := vProvider.Load(context.Background()); err != nil {
			log.Fatalln(err)
		}

		// Should be able to call a command with the loaded secrets.
		c := exec.Command(command, arguments...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin

		if err := c.Run(); err != nil {
			log.Fatalln(err)
		}
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

	vaultCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}} -- [command to run] [command args]{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)
}
