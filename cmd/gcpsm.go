// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/gcpsm"
	"github.com/thalesfsp/configurer/option"
)

var newGCPSMProvider = gcpsm.New

// gcpsmCmd represents the gcpsm command.
var gcpsmCmd = &cobra.Command{
	Aliases: []string{"gsm"},
	Short:   "GCP Secret Manager provider",
	Use:     "gcpsm",
	Example: "  configurer l gcpsm -p my-project -s my-secret -- env",
	Long: `GCP Secret Manager provider will load secrets from Google Cloud Secret Manager,
export them to the environment, and then run, if any, the specified command.

The provider supports both JSON and plain text secrets. For JSON secrets,
each key-value pair will be exported as a separate environment variable.
For plain text secrets, the secret name (last part after /) will be used
as the environment variable name.

Authentication uses Google Application Default Credentials. Set
GOOGLE_APPLICATION_CREDENTIALS when a service-account credential file is
needed.

The following environment variables can be used to configure the provider:
- GCP_PROJECT_ID: The Google Cloud project containing the secrets.
- GOOGLE_CLOUD_PROJECT: Fallback Google Cloud project.
- GCPSM_SECRET_NAME: The secret name to load.

NOTE: Already exported environment variables have precedence over loaded
      ones. Set the override flag to true to override them.`,
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("project-id").Value.String() == "" {
			log.Fatalln("--project-id is required")
		}

		if cmd.Flag("secret-name").Value.String() == "" {
			log.Fatalln("--secret-name is required")
		}

		shouldOverride := cmd.Flag("override").Value.String() == "true"
		rawValue := cmd.Flag("rawValue").Value.String() == "true"

		config := &gcpsm.Config{
			ProjectID: cmd.Flag("project-id").Value.String(),
		}
		secretInformation := &gcpsm.SecretInformation{
			SecretNames: []string{cmd.Flag("secret-name").Value.String()},
		}

		gcpsmProvider, err := newGCPSMProvider(
			shouldOverride,
			rawValue,
			config,
			secretInformation,
		)
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

		finalValues, err := gcpsmProvider.Load(context.Background(), options...)
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

		ConcurrentRunner(gcpsmProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(gcpsmCmd)

	gcpsmCmd.Flags().StringP(
		"project-id",
		"p",
		gcpProjectID(),
		"Google Cloud project containing the secrets",
	)
	gcpsmCmd.Flags().StringP(
		"secret-name",
		"s",
		os.Getenv("GCPSM_SECRET_NAME"),
		"Secret name to load from Google Cloud Secret Manager",
	)

	gcpsmCmd.SetUsageTemplate(providerUsageTemplate)
}

func gcpProjectID() string {
	if projectID := os.Getenv("GCP_PROJECT_ID"); projectID != "" {
		return projectID
	}

	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}
