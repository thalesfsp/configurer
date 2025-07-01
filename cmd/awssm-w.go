// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/awssm"
	"github.com/thalesfsp/configurer/util"
)

// awssmWCmd represents the awssm write command.
var awssmWCmd = &cobra.Command{
	Aliases: []string{"a"},
	Short:   "AWS Secrets Manager provider",
	Use:     "awssm",
	Example: "  configurer w --source dev.env awssm -r us-east-1 -n myapp/prod/secrets",
	Long: `AWS Secrets Manager provider will write secrets to AWS Secrets Manager.

The provider supports storing configurations as JSON secrets in AWS Secrets Manager.
All key-value pairs from the source file will be stored as a single JSON object
in the specified secret.

It supports the following authentication methods:
- Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
- AWS credentials file (~/.aws/credentials)
- IAM roles (recommended for EC2/ECS/Lambda)
- AWS SSO

The following environment variables can be used to configure the provider:
- AWS_REGION: The AWS region where secrets are stored.
- AWS_PROFILE: The AWS profile to use for authentication.
- AWS_ACCESS_KEY_ID: The AWS access key ID (not recommended, use IAM roles instead).
- AWS_SECRET_ACCESS_KEY: The AWS secret access key (not recommended, use IAM roles instead).
- AWSSM_SECRET_NAME: The secret name to write to AWS Secrets Manager.

NOTE: If the secret does not exist, it will be created. If it exists, it will be updated.
NOTE: It's recommended to use IAM roles for authentication instead of access keys.

## Setting up the environment to run the application

There are two methods to set up the environment to run the application.

### Flags (not recommended)

Values are set by specifying flags. In the following example, values are
set and configuration is written to AWS Secrets Manager.

  configurer w --source dev.env awssm \
      --region        "us-east-1" \
      --secret-name  "myapp/prod/secrets" \
      --profile       "default"

### Environment Variables (this is the recommended, and preferred way)

Setup values are set by specifying environment variables. In the following
example, values are set and configuration is written. It's cleaner and
more secure.

  export AWS_REGION="us-east-1"
  export AWS_PROFILE="default"
  export AWSSM_SECRET_NAME="myapp/prod/secrets"

  configurer w --source dev.env awssm`,
	Run: func(cmd *cobra.Command, _ []string) {
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

		//////
		// Validation.
		//////

		region := cmd.Flag("region").Value.String()
		profile := cmd.Flag("profile").Value.String()
		accessKey := cmd.Flag("access-key").Value.String()
		secretKey := cmd.Flag("secret-key").Value.String()

		// Ensure if profile is specified, access key and secret key are not specified.
		if profile != "" && (accessKey != "" || secretKey != "") {
			log.Fatalln("if --profile is specified, --access-key and --secret-key must not be specified")
		}

		// Ensure if access key or secret key are specified, profile is not specified.
		if (accessKey != "" || secretKey != "") && profile != "" {
			log.Fatalln("if --access-key or --secret-key is specified, --profile must not be specified")
		}

		// Ensure if profile is not specified, region must be specified.
		if profile == "" && region == "" {
			log.Fatalln("if --profile is not specified, --region must be specified")
		}

		// Ensure if access key is specified, secret key must also be specified.
		if accessKey == "" && secretKey != "" {
			log.Fatalln("if --secret-key is specified, --access-key must also be specified")
		}

		// Ensure if secret key is specified, access key must also be specified.
		if secretKey == "" && accessKey != "" {
			log.Fatalln("if --access-key is specified, --secret-key must also be specified")
		}

		//////
		// Build config.
		//////

		config := &awssm.Config{}

		if profile != "" {
			config.Profile = profile

			if region != "" {
				config.Region = region
			}
		}

		if accessKey != "" && secretKey != "" {
			config.AccessKey = accessKey
			config.SecretKey = secretKey
			config.Region = region
		}

		if profile == "" && accessKey == "" && secretKey == "" {
			config.Region = region
		}

		//////
		// Handle secret name from flag.
		//////

		secretName := cmd.Flag("secret-name").Value.String()

		sI := &awssm.SecretInformation{
			SecretNames: []string{secretName},
		}

		awssmProvider, err := awssm.New(false, false, config, sI)
		if err != nil {
			log.Fatalln(err)
		}

		if err := awssmProvider.Write(ctx, parsedFile); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(awssmWCmd)

	// Connection.
	awssmWCmd.Flags().StringP("region", "r", os.Getenv("AWS_REGION"), "AWS region where secrets are stored")
	awssmWCmd.Flags().StringP("profile", "p", os.Getenv("AWS_PROFILE"), "AWS profile to use for authentication")

	// Auth.
	awssmWCmd.Flags().String("access-key", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS access key ID (not recommended)")
	awssmWCmd.Flags().String("secret-key", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS secret access key (not recommended)")

	// Secret.
	awssmWCmd.Flags().StringP("secret-name", "n", os.Getenv("AWSSM_SECRET_NAME"), "Secret name to write to AWS Secrets Manager")

	awssmWCmd.MarkFlagRequired("secret-name")

	awssmWCmd.SetUsageTemplate(providerUsageTemplate)
}
