// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/util"
)

// k8sSecretWCmd represents the Kubernetes Secrets write command.
var k8sSecretWCmd = &cobra.Command{
	Aliases: []string{"k8s"},
	Short:   "Kubernetes Secrets provider",
	Use:     "k8ssecret",
	Example: "  configurer w --source dev.env k8ssecret --secret-name app --token token",
	Long: `Kubernetes Secrets provider will write secrets from a source file to
the Kubernetes API. If the target Secret does not exist, it is created.

The following environment variables can configure the provider:
- K8S_API_SERVER: Kubernetes API server.
- K8S_NAMESPACE: Kubernetes namespace.
- K8S_SECRET_NAME: Kubernetes Secret name.
- K8S_TOKEN: Kubernetes bearer token.`,
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

		config := k8sSecretConfig(cmd)

		kubernetesSecretProvider, err := newK8sSecretProvider(false, false, config)
		if err != nil {
			log.Fatalln(err)
		}

		if err := kubernetesSecretProvider.Write(ctx, values); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(k8sSecretWCmd)

	addK8sSecretFlags(k8sSecretWCmd)

	k8sSecretWCmd.SetUsageTemplate(providerUsageTemplate)
}
