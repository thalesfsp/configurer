// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/doppler"
	"github.com/thalesfsp/configurer/option"
)

var newDopplerProvider = doppler.New

// dopplerCmd represents the Doppler load command.
var dopplerCmd = &cobra.Command{
	Aliases: []string{"dp"},
	Short:   "Doppler provider",
	Use:     "doppler",
	Example: "  configurer l doppler --token dp.pt.token --project app --config development -- env",
	Long: `Doppler provider will load secrets from Doppler, export them to the
environment, and then run, if any, the specified command.

The following environment variables can configure the provider:
- DOPPLER_TOKEN: Doppler token.
- DOPPLER_PROJECT: Doppler project.
- DOPPLER_CONFIG: Doppler config.

Project and config are optional when using a Doppler service token.

NOTE: Already exported environment variables have precedence over loaded
      ones. Set the override flag to true to override them.`,
	Run: func(cmd *cobra.Command, args []string) {
		shouldOverride := cmd.Flag("override").Value.String() == "true"
		rawValue := cmd.Flag("rawValue").Value.String() == "true"

		//////
		// Build config.
		//////

		config := &doppler.Config{
			Token:   cmd.Flag("token").Value.String(),
			Project: cmd.Flag("project").Value.String(),
			Config:  cmd.Flag("config").Value.String(),
		}

		dopplerProvider, err := newDopplerProvider(shouldOverride, rawValue, config)
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

		finalValues, err := dopplerProvider.Load(context.Background(), options...)
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

		ConcurrentRunner(dopplerProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(dopplerCmd)

	dopplerCmd.Flags().StringP("token", "t", os.Getenv("DOPPLER_TOKEN"), "Doppler token")
	dopplerCmd.Flags().StringP("project", "p", os.Getenv("DOPPLER_PROJECT"), "Doppler project")
	dopplerCmd.Flags().String("config", os.Getenv("DOPPLER_CONFIG"), "Doppler config")

	dopplerCmd.SetUsageTemplate(providerUsageTemplate)
}
