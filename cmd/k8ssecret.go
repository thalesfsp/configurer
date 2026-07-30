// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/k8ssecret"
	"github.com/thalesfsp/configurer/option"
)

var newK8sSecretProvider = k8ssecret.New

// k8sSecretCmd represents the Kubernetes Secrets load command.
var k8sSecretCmd = &cobra.Command{
	Aliases: []string{"k8s"},
	Short:   "Kubernetes Secrets provider",
	Use:     "k8ssecret",
	Example: "  configurer l k8ssecret --path /var/run/secrets/app -- env",
	Long: `Kubernetes Secrets provider will load secrets from a mounted Secret
directory or the Kubernetes API, export them to the environment, and then run,
if any, the specified command.

The following environment variables can configure the provider:
- K8S_SECRET_PATH: Mounted Secret directory.
- K8S_API_SERVER: Kubernetes API server.
- K8S_NAMESPACE: Kubernetes namespace.
- K8S_SECRET_NAME: Kubernetes Secret name.
- K8S_TOKEN: Kubernetes bearer token.

NOTE: Already exported environment variables have precedence over loaded
      ones. Set the override flag to true to override them.`,
	Run: func(cmd *cobra.Command, args []string) {
		shouldOverride := cmd.Flag("override").Value.String() == "true"
		rawValue := cmd.Flag("rawValue").Value.String() == "true"

		//////
		// Build config.
		//////

		config := k8sSecretConfig(cmd)

		kubernetesSecretProvider, err := newK8sSecretProvider(shouldOverride, rawValue, config)
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

		finalValues, err := kubernetesSecretProvider.Load(context.Background(), options...)
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

		ConcurrentRunner(kubernetesSecretProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(k8sSecretCmd)

	addK8sSecretFlags(k8sSecretCmd)

	k8sSecretCmd.SetUsageTemplate(providerUsageTemplate)
}

func addK8sSecretFlags(command *cobra.Command) {
	command.Flags().StringP(
		"path",
		"p",
		os.Getenv("K8S_SECRET_PATH"),
		"Mounted Kubernetes Secret directory",
	)
	command.Flags().String(
		"api-server",
		os.Getenv("K8S_API_SERVER"),
		"Kubernetes API server",
	)
	command.Flags().StringP(
		"namespace",
		"n",
		k8sNamespace(),
		"Kubernetes namespace",
	)
	command.Flags().String(
		"secret-name",
		os.Getenv("K8S_SECRET_NAME"),
		"Kubernetes Secret name",
	)
	command.Flags().StringP(
		"token",
		"t",
		os.Getenv("K8S_TOKEN"),
		"Kubernetes bearer token",
	)
	command.Flags().String(
		"token-file",
		"",
		"Kubernetes bearer token file",
	)
	command.Flags().String(
		"ca-cert-file",
		"",
		"Kubernetes CA certificate file",
	)
	command.Flags().Bool(
		"insecure-skip-tls-verify",
		false,
		"Skip Kubernetes API TLS certificate verification",
	)
}

func k8sSecretConfig(command *cobra.Command) *k8ssecret.Config {
	return &k8ssecret.Config{
		Path:                  command.Flag("path").Value.String(),
		APIServer:             command.Flag("api-server").Value.String(),
		Namespace:             command.Flag("namespace").Value.String(),
		SecretName:            command.Flag("secret-name").Value.String(),
		Token:                 command.Flag("token").Value.String(),
		TokenFile:             command.Flag("token-file").Value.String(),
		CACertFile:            command.Flag("ca-cert-file").Value.String(),
		InsecureSkipTLSVerify: command.Flag("insecure-skip-tls-verify").Value.String() == "true",
	}
}

func k8sNamespace() string {
	if namespace := os.Getenv("K8S_NAMESPACE"); namespace != "" {
		return namespace
	}

	return "default"
}
