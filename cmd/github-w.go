package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/github"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/configurer/util"
)

var (
	githubWriteEnvironment string
	githubWriteHTTPVerb    string
	githubWriteOwner       string
	githubWriteRepo        string
	githubWriteTarget      string
	githubWriteVariable    bool
)

var newGitHubProvider = func(
	override, rawValue bool,
	owner, repository string,
) (provider.IProvider, error) {
	return github.New(override, rawValue, owner, repository)
}

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

		p, err := newGitHubProvider(false, false, githubWriteOwner, githubWriteRepo)
		if err != nil {
			log.Fatalln(err)
		}

		var opts []option.WriteFunc

		if githubWriteEnvironment != "" {
			opts = append(opts, option.WithEnvironment(githubWriteEnvironment))
		}

		if githubWriteVariable {
			opts = append(opts, option.WithVariable(githubWriteVariable))
		}

		if githubWriteTarget != "" {
			opts = append(opts, option.WithTarget(githubWriteTarget))
		}

		if githubWriteHTTPVerb != "" {
			opts = append(opts, option.WithHTTPVerb(githubWriteHTTPVerb))
		}

		if err := p.Write(ctx, parsedFile, opts...); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(githubWCmd)

	githubWCmd.Flags().StringVarP(&githubWriteOwner, "owner", "o", "", "owner of the repository")
	githubWCmd.Flags().StringVarP(&githubWriteRepo, "repo", "p", "", "repository name")
	githubWCmd.Flags().StringVar(&githubWriteEnvironment, "environment", "", "environment to write secrets")
	githubWCmd.Flags().BoolVar(&githubWriteVariable, "variable", false, "variable to write secrets")
	githubWCmd.Flags().StringVar(&githubWriteTarget, "target", github.Actions.String(), "target to write secrets, e.g.: codespaces, actions")
	githubWCmd.Flags().StringVar(&githubWriteHTTPVerb, "httpVerb", "", "HTTP verb to be used")

	githubWCmd.MarkFlagRequired("owner")
	githubWCmd.MarkFlagRequired("repo")

	githubWCmd.SetUsageTemplate(providerUsageTemplate)
}
