package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/github"
	"github.com/thalesfsp/configurer/internal/logging"
	"github.com/thalesfsp/configurer/parsers/env"
	"github.com/thalesfsp/sypl/level"
)

var environment, owner, repo string

// githubWCmd represents the env command.
var githubWCmd = &cobra.Command{
	Aliases: []string{"g"},
	Short:   "GitHub provider",
	Use:     "github",
	Example: "  configurer w --source prod.env g -o owner -p repo",
	Long: `GitHub provider will write secrets to GitHub Secrets

The following environment variables can be used to configure the provider:
- GITHUB_TOKEN: The token to use for authentication.`,
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

		p, err := github.New(false, owner, repo)
		if err != nil {
			log.Fatalln(err)
		}

		if err := p.Write(ctx, parsedFile); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(githubWCmd)

	githubWCmd.Flags().StringVarP(&owner, "owner", "o", "", "owner of the repository")
	githubWCmd.Flags().StringVarP(&repo, "repo", "p", "", "repository name")
	githubWCmd.Flags().StringVarP(&environment, "environment", "e", "", "environment to write secrets")

	githubWCmd.MarkFlagRequired("owner")
	githubWCmd.MarkFlagRequired("repo")

	githubWCmd.SetUsageTemplate(providerUsageTemplate)
}
