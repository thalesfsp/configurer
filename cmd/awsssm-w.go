// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/awsssm"
	"github.com/thalesfsp/configurer/util"
)

// awsssmWriteCmd represents the awsssm write command.
var awsssmWriteCmd = &cobra.Command{
	Aliases: []string{"ssm"},
	Short:   "AWS SSM Parameter Store provider (write)",
	Use:     "awsssm",
	Example: "  configurer w --source dev.env awsssm -r us-east-1 --path /myapp/prod",
	Long: `AWS SSM Parameter Store provider (write) will store parameters in AWS Systems
Manager Parameter Store.

Parameters are written as SecureString by default for security. They are
stored under the specified path prefix. Each key-value pair from the source
file becomes a separate parameter.

For example, with --path /myapp/prod and source containing DB_HOST=localhost:
  -> Creates parameter /myapp/prod/DB_HOST with value "localhost"

The provider supports the following authentication methods:
- Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
- AWS credentials file (~/.aws/credentials)
- IAM roles (recommended for EC2/ECS/Lambda)
- AWS SSO

The following environment variables can be used to configure the provider:
- AWS_REGION: The AWS region where parameters are stored.
- AWS_PROFILE: The AWS profile to use for authentication.
- AWSSSM_PATH: The parameter path prefix for writing.

NOTE: It's recommended to use IAM roles for authentication instead of access keys.

## Setting up the environment to run the application

There are two methods to set up the environment to run the application.

### Flags (not recommended)

Values are set by specifying flags. In the following example, values are
set and configuration is written to AWS SSM Parameter Store.

  configurer w --source dev.env awsssm \
      --region  "us-east-1" \
      --path    "/myapp/prod" \
      --profile "default"

### Environment Variables (this is the recommended, and preferred way)

Setup values are set by specifying environment variables. In the following
example, values are set and configuration is written. It's cleaner and
more secure.

  export AWS_REGION="us-east-1"
  export AWS_PROFILE="default"
  export AWSSSM_PATH="/myapp/prod"

  configurer w --source dev.env awsssm`,
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

		config := &awsssm.Config{}

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

		// When no profile or access keys, still set region from flag.
		if profile == "" && accessKey == "" && secretKey == "" {
			config.Region = region
		}

		//////
		// Handle path.
		//////

		path := cmd.Flag("path").Value.String()
		if path == "" {
			log.Fatalln("--path is required for write operations")
		}

		paramInfo := &awsssm.ParameterInformation{
			Path:           path,
			WithDecryption: true,
		}

		awsssmProvider, err := awsssm.New(false, false, config, paramInfo)
		if err != nil {
			log.Fatalln(err)
		}

		if err := awsssmProvider.Write(ctx, parsedFile); err != nil {
			log.Fatalln(err)
		}

		os.Exit(0)
	},
}

func init() {
	writeCmd.AddCommand(awsssmWriteCmd)

	// Connection.
	awsssmWriteCmd.Flags().StringP("region", "r", os.Getenv("AWS_REGION"), "AWS region where parameters are stored")
	awsssmWriteCmd.Flags().StringP("profile", "p", os.Getenv("AWS_PROFILE"), "AWS profile to use for authentication")

	// Auth.
	awsssmWriteCmd.Flags().String("access-key", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS access key ID (not recommended)")
	awsssmWriteCmd.Flags().String("secret-key", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS secret access key (not recommended)")

	// Path.
	awsssmWriteCmd.Flags().String("path", os.Getenv("AWSSSM_PATH"), "Parameter path prefix for writing (e.g., /myapp/prod)")

	awsssmWriteCmd.SetUsageTemplate(providerUsageTemplate)
}
