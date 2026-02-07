// Package cmd provides the CLI commands for the configurer application.
package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/configurer/awsssm"
	"github.com/thalesfsp/configurer/option"
)

// awsssmCmd represents the awsssm command.
var awsssmCmd = &cobra.Command{
	Aliases: []string{"awsssm", "ssm"},
	Short:   "AWS SSM Parameter Store provider",
	Use:     "awsssm",
	Example: `  # Load all parameters under a path
  configurer l awsssm -r us-east-1 --path /myapp/prod -- env

  # Load a specific parameter
  configurer l awsssm -r us-east-1 --parameter-name /myapp/prod/DB_HOST -- env

  # Load parameters recursively
  configurer l awsssm -r us-east-1 --path /myapp --recursive -- env`,
	Long: `AWS SSM Parameter Store provider will load parameters from AWS Systems Manager
Parameter Store, export them to the environment, and then run, if any,
the specified command.

Parameter Store supports three parameter types:
- String: Plain text values
- StringList: Comma-separated values
- SecureString: Encrypted values (decrypted automatically by default)

Parameters can be loaded in two ways:
1. By path prefix: All parameters under a path (e.g., /myapp/prod/*)
2. By specific names: Individual parameter names

For path-based loading, the parameter name (last segment) becomes the
environment variable name. For example:
  /myapp/prod/DB_HOST -> DB_HOST
  /myapp/prod/DB_PORT -> DB_PORT

It supports the following authentication methods:
- Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
- AWS credentials file (~/.aws/credentials)
- IAM roles (recommended for EC2/ECS/Lambda)
- AWS SSO

The following environment variables can be used to configure the provider:
- AWS_REGION: The AWS region where parameters are stored.
- AWS_PROFILE: The AWS profile to use for authentication.
- AWS_ACCESS_KEY_ID: The AWS access key ID (not recommended, use IAM roles instead).
- AWS_SECRET_ACCESS_KEY: The AWS secret access key (not recommended, use IAM roles instead).
- AWSSSM_PATH: The parameter path prefix to load.
- AWSSSM_PARAMETER_NAME: A specific parameter name to load.

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
Example: configurer l awsssm -r us-east-1 --path /myapp/prod -c "ls -la" -c "env"

NOTE: Already exported environment variables have precedence over loaded
      ones. Set the overwrite flag to true to override them.

NOTE: The "-c" flag have precedence over double dash (--)

## Setting up the environment to run the application

There are two methods to set up the environment to run the application.

### Flags (not recommended)

Values are set by specifying flags. In the following example, values are
set and then the env command is run.

  configurer l awsssm \
      --region         "us-east-1" \
      --path           "/myapp/prod" \
      --profile        "default" -- env

### Environment Variables (this is the recommended, and preferred way)

Setup values are set by specifying environment variables. In the following
example, values are set and then the env command is run. It's cleaner and
more secure.

  export AWS_REGION="us-east-1"
  export AWS_PROFILE="default"
  export AWSSSM_PATH="/myapp/prod"

  configurer l awsssm -- env`,
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

		//////
		// Handle parameter info from flags or environment variables.
		//////

		path := cmd.Flag("path").Value.String()
		paramName := cmd.Flag("parameter-name").Value.String()
		recursive := cmd.Flag("recursive").Value.String() == "true"
		noDecrypt := cmd.Flag("no-decrypt").Value.String() == "true"

		// Validate that at least one of path or parameter-name is specified.
		if path == "" && paramName == "" {
			log.Fatalln("either --path or --parameter-name must be specified")
		}

		paramInfo := &awsssm.ParameterInformation{
			Path:           path,
			Recursive:      recursive,
			WithDecryption: !noDecrypt,
		}

		if paramName != "" {
			paramInfo.ParameterNames = []string{paramName}
		}

		awsssmProvider, err := awsssm.New(shouldOverride, rawValue, config, paramInfo)
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

		finalValues, err := awsssmProvider.Load(context.Background(), options...)
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

		ConcurrentRunner(awsssmProvider, commands, args)
	},
}

func init() {
	loadCmd.AddCommand(awsssmCmd)

	// Connection.
	awsssmCmd.Flags().StringP("region", "r", os.Getenv("AWS_REGION"), "AWS region where parameters are stored")
	awsssmCmd.Flags().StringP("profile", "p", os.Getenv("AWS_PROFILE"), "AWS profile to use for authentication")

	// Auth.
	awsssmCmd.Flags().String("access-key", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS access key ID (not recommended)")
	awsssmCmd.Flags().String("secret-key", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS secret access key (not recommended)")

	// Parameters.
	awsssmCmd.Flags().String("path", os.Getenv("AWSSSM_PATH"), "Parameter path prefix to load (e.g., /myapp/prod)")
	awsssmCmd.Flags().String("parameter-name", os.Getenv("AWSSSM_PARAMETER_NAME"), "Specific parameter name to load")
	awsssmCmd.Flags().Bool("recursive", true, "Recursively load parameters under path (default: true)")
	awsssmCmd.Flags().Bool("no-decrypt", false, "Do not decrypt SecureString parameters")

	awsssmCmd.SetUsageTemplate(providerUsageTemplate)
}
