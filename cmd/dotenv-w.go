package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/dotenv"
	"github.com/thalesfsp/configurer/internal/logging"
	"github.com/thalesfsp/configurer/parsers/env"
	"github.com/thalesfsp/sypl/level"
)

var targetFilename string

// dotEnvWCmd represents the env command.
var dotEnvWCmd = &cobra.Command{
	Aliases: []string{"d"},
	Short:   "DotEnv provider",
	Use:     "dotenv",
	Example: "  configurer w --source prod.env l --target .env",
	Long:    "DotEnv provider will write secrets to a `*.env` file",
	Run: func(cmd *cobra.Command, args []string) {
		if logging.Get().AnyMaxLevel(level.Debug) {
			logging.Get().Breakpoint(env.Name)
		}

		// Context with timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		f, err := os.Open(sourceFilename)
		if err != nil {
			log.Fatalln(err)
		}

		parsedFile, err := ParseFile(ctx, f)
		if err != nil {
			log.Fatalln(err)
		}

		dotEnvProvider, err := dotenv.New(false, dotEnvFiles...)
		if err != nil {
			log.Fatalln(err)
		}

		if err := dotEnvProvider.Write(ctx, parsedFile); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(dotEnvWCmd)

	dotEnvWCmd.Flags().StringVarP(&targetFilename, "target", "t", ".env", "The dot env file to write")

	dotEnvWCmd.MarkFlagRequired("target")

	dotEnvWCmd.SetUsageTemplate(providerUsageTemplate)
}
