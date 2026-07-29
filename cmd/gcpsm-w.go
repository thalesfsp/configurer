// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/gcpsm"
	"github.com/thalesfsp/configurer/util"
)

// gcpsmWCmd represents the gcpsm write command.
var gcpsmWCmd = &cobra.Command{
	Short:   "GCP Secret Manager provider",
	Use:     "gcpsm",
	Example: "  configurer w --source dev.env gcpsm -p my-project -n my-secret",
	Long: `GCP Secret Manager provider will write secrets to Google Cloud Secret Manager.

All key-value pairs from the source file are stored as a single JSON object.
	Authentication uses Google Application Default Credentials. If the target
secret does not exist, it is created with automatic replication.`,
	Run: func(cmd *cobra.Command, _ []string) {
		if cmd.Flag("project-id").Value.String() == "" {
			log.Fatalln("--project-id is required")
		}

		if cmd.Flag("secret-name").Value.String() == "" {
			log.Fatalln("--secret-name is required")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		file, err := os.Open(sourceFilename)
		if err != nil {
			log.Fatalln(err)
		}
		defer file.Close()

		parsedFile, err := util.ParseFile(ctx, file)
		if err != nil {
			log.Fatalln(err)
		}

		config := &gcpsm.Config{
			ProjectID: cmd.Flag("project-id").Value.String(),
		}
		secretInformation := &gcpsm.SecretInformation{
			SecretNames: []string{cmd.Flag("secret-name").Value.String()},
		}

		gcpsmProvider, err := newGCPSMProvider(false, false, config, secretInformation)
		if err != nil {
			log.Fatalln(err)
		}

		if err := gcpsmProvider.Write(ctx, parsedFile); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(gcpsmWCmd)

	gcpsmWCmd.Flags().StringP(
		"project-id",
		"p",
		gcpProjectID(),
		"Google Cloud project containing the secret",
	)
	gcpsmWCmd.Flags().StringP(
		"secret-name",
		"n",
		os.Getenv("GCPSM_SECRET_NAME"),
		"Secret name to write to Google Cloud Secret Manager",
	)

	gcpsmWCmd.SetUsageTemplate(providerUsageTemplate)
}
