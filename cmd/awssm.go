// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/awssm"
	"github.com/thalesfsp/configurer/option"
)

// awssmCmd represents the awssm command.
var awssmCmd = &cobra.Command{
	Aliases: []string{"awssm", "asm"},
	Short:   "AWS Secrets Manager provider",
	Use:     "awssm",
	Example: "  configurer l awssm -r us-east-1 -s myapp/prod/secrets -- env",
	Long: `AWS Secrets Manager provider will load secrets from AWS Secrets Manager,
export them to the environment, and then run, if any, the specified
command.

The provider supports both JSON and plain text secrets. For JSON secrets,
each key-value pair will be exported as a separate environment variable.
For plain text secrets, the secret name (last part after /) will be used
as the environment variable name.

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
- AWSSM_SECRET_NAME: The secret name to load from AWS Secrets Manager.

NOTE: It's recommended to use IAM roles for authentication instead of access keys.

## About the command to run

If running only one command:
- The <command> to run must be the last argument.
- The command <arguments> must be after the <command>.
- The <command> will inherit the environment variables from the
parent process plus the ones loaded from the provider.

NOTE: A double dash (--) is used to signify the end of command options.
      It's required to distinguish between the flags passed to Go and
	  those that aren't. Everything after the double dash won't be
	  considered to be Go's flags.

If running multiple commands:
Use as many -c flags you want, to specify the commands to run.
The commands will be run concurrently.
Example: configurer l awssm -r us-east-1 -s myapp/prod/secrets -c "ls -la" -c "env"

NOTE: Already exported environment variables have precedence over loaded
      ones. Set the overwrite flag to true to override them.

NOTE: The "-c" flag have precedence over double dash (--)

## Setting up the environment to run the application

There are two methods to set up the environment to run the application.

### Flags (not recommended)

Values are set by specifying flags. In the following example, values are
set and then the env command is run.

  configurer l awssm \
      --region       "us-east-1" \
      --secret-name  "myapp/prod/secrets" \
      --profile      "default" -- env

### Environment Variables (this is the recommended, and preferred way)

Setup values are set by specifying environment variables. In the following
example, values are set and then the env command is run. It's cleaner and
more secure.

  export AWS_REGION="us-east-1"
  export AWS_PROFILE="default"
  export AWSSM_SECRET_NAME="myapp/prod/secrets"

  configurer l awssm -- env`,
	Run: func(cmd *cobra.Command, args []string) {
		// Should be able to override current environment variables.
		shouldOverride := cmd.Flag("override").Value.String() == "true"

		rawValue := cmd.Flag("rawValue").Value.String() == "true"

		//////
		// Validation.
		//////

		region := cmd.Flag("region").Value.String()
		profile := cmd.Flag("profile").Value.String()
		accessKey := cmd.Flag("access-key").Value.String()
		secretKey := cmd.Flag("secret-key").Value.String()

		// Ensure if profile is specified, access key and secret key are not specified.
		if profile != "" && (accessKey != "" || secretKey != "") {
			log.Fatalln("if --profile is specified, --access-key and --secret key must not be specified")
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

		//////
		// Handle secret names from flag or environment variable.
		//////

		secretName := cmd.Flag("secret-name").Value.String()

		sI := &awssm.SecretInformation{
			SecretNames: []string{secretName},
		}

		awssmProvider, err := awssm.New(shouldOverride, rawValue, config, sI)
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

		finalValues, err := awssmProvider.Load(context.Background(), options...)
		if err != nil {
			log.Fatalln(err)
		}

		// Should be able to dump the loaded values to a file.
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

		ConcurrentRunner(awssmProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(awssmCmd)

	// Connection.
	awssmCmd.Flags().StringP("region", "r", os.Getenv("AWS_REGION"), "AWS region where secrets are stored")
	awssmCmd.Flags().StringP("profile", "p", os.Getenv("AWS_PROFILE"), "AWS profile to use for authentication")

	// Auth.
	awssmCmd.Flags().String("access-key", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS access key ID (not recommended)")
	awssmCmd.Flags().String("secret-key", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS secret access key (not recommended)")

	// Secret.
	awssmCmd.Flags().StringP("secret-name", "s", os.Getenv("AWSSM_SECRET_NAME"), "Secret name to load from AWS Secrets Manager")
	awssmCmd.MarkFlagRequired("secret-name")

	awssmCmd.SetUsageTemplate(providerUsageTemplate)
}
