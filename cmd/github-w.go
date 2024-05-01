package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/github"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/util"
)

var (
	environment, owner, repo string
	httpVerb                 string
	target                   string
	variable                 bool
)

// githubWCmd represents the env command.
var githubWCmd = &cobra.Command{
	Aliases: []string{"g"},
	Short:   "GitHub provider",
	Use:     "github",
	Example: "  configurer w --source prod.env g -o owner -p repo",
	Long: `GitHub provider will write secrets to GitHub Secrets

The following environment variables can be used to configure the provider:
- GITHUB_TOKEN: The token to use for authentication.

NOTES: 
- Your token needs to have write access to the repository AND be able
to read your public key.
- If you are using "environment" flag, you need to create the environment.
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Context with timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		f, err := os.Open(sourceFilename)
		if err != nil {
			log.Fatalln(err)
		}

		parsedFile, err := util.ParseFile(ctx, f)
		if err != nil {
			log.Fatalln(err)
		}

		p, err := github.New(false, false, owner, repo)
		if err != nil {
			log.Fatalln(err)
		}

		var opts []option.WriteFunc

		if environment != "" {
			opts = append(opts, option.WithEnvironment(environment))
		}

		if variable {
			opts = append(opts, option.WithVariable(variable))
		}

		if target != "" {
			opts = append(opts, option.WithTarget(target))
		}

		if httpVerb != "" {
			opts = append(opts, option.WithHTTPVerb(httpVerb))
		}

		if err := p.Write(ctx, parsedFile, opts...); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(githubWCmd)

	githubWCmd.Flags().StringVarP(&owner, "owner", "o", "", "owner of the repository")
	githubWCmd.Flags().StringVarP(&repo, "repo", "p", "", "repository name")
	githubWCmd.Flags().StringVar(&environment, "environment", "", "environment to write secrets")
	githubWCmd.Flags().BoolVar(&variable, "variable", false, "variable to write secrets")
	githubWCmd.Flags().StringVar(&target, "target", github.Actions.String(), "target to write secrets, e.g.: codespaces, actions")
	githubWCmd.Flags().StringVar(&httpVerb, "httpVerb", "", "HTTP verb to be used")

	githubWCmd.MarkFlagRequired("owner")
	githubWCmd.MarkFlagRequired("repo")

	githubWCmd.SetUsageTemplate(providerUsageTemplate)
}
