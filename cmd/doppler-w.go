// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/doppler"
	"github.com/thalesfsp/configurer/util"
)

// dopplerWCmd represents the Doppler write command.
var dopplerWCmd = &cobra.Command{
	Short:   "Doppler provider",
	Use:     "doppler",
	Example: "  configurer w --source dev.env doppler --token dp.pt.token --project app --config development",
	Long: `Doppler provider will write secrets from a source file to Doppler.

The following environment variables can configure the provider:
- DOPPLER_TOKEN: Doppler token.
- DOPPLER_PROJECT: Doppler project.
- DOPPLER_CONFIG: Doppler config.

Project and config are optional when using a Doppler service token.`,
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

		config := &doppler.Config{
			Token:   cmd.Flag("token").Value.String(),
			Project: cmd.Flag("project").Value.String(),
			Config:  cmd.Flag("config").Value.String(),
		}

		dopplerProvider, err := newDopplerProvider(false, false, config)
		if err != nil {
			log.Fatalln(err)
		}

		if err := dopplerProvider.Write(ctx, values); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(dopplerWCmd)

	dopplerWCmd.Flags().StringP("token", "t", os.Getenv("DOPPLER_TOKEN"), "Doppler token")
	dopplerWCmd.Flags().StringP("project", "p", os.Getenv("DOPPLER_PROJECT"), "Doppler project")
	dopplerWCmd.Flags().String("config", os.Getenv("DOPPLER_CONFIG"), "Doppler config")

	dopplerWCmd.SetUsageTemplate(providerUsageTemplate)
}
