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

// awsssmWriteCmd represents the awsssm write command.
var awsssmWriteCmd = &cobra.Command{
	Aliases: []string{"awsssm", "ssm"},
	Short:   "AWS SSM Parameter Store provider (write)",
	Use:     "awsssm",
	Example: `  # Write parameters under a path
  configurer w awsssm -r us-east-1 --path /myapp/prod -v DB_HOST=localhost -v DB_PORT=5432

  # Write from a file
  configurer w awsssm -r us-east-1 --path /myapp/prod -f config.env`,
	Long: `AWS SSM Parameter Store provider (write) will store parameters in AWS Systems
Manager Parameter Store.

Parameters are written as SecureString by default for security. They are
stored under the specified path prefix.

For example, with --path /myapp/prod and -v DB_HOST=localhost:
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

## Values to write

Values can be specified in two ways:

### Using -v flag

  configurer w awsssm --path /myapp/prod -v KEY1=value1 -v KEY2=value2

### Using a file

  configurer w awsssm --path /myapp/prod -f config.env

The file should be in .env format (KEY=value per line).`,
	Run: func(cmd *cobra.Command, args []string) {
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

		//////
		// Collect values to write.
		//////

		valuesToWrite := make(map[string]interface{})

		// Get values from -v flags.
		values, err := cmd.Flags().GetStringSlice("value")
		if err != nil {
			log.Fatalln(err)
		}

		for _, v := range values {
			parts := splitKeyValue(v)
			if len(parts) == 2 {
				valuesToWrite[parts[0]] = parts[1]
			}
		}

		// Get values from file if specified.
		filename, _ := cmd.Flags().GetString("file")
		if filename != "" {
			fileValues, err := loadValuesFromFile(filename)
			if err != nil {
				log.Fatalln(err)
			}

			for k, v := range fileValues {
				valuesToWrite[k] = v
			}
		}

		if len(valuesToWrite) == 0 {
			log.Fatalln("no values to write; use -v KEY=value or -f filename")
		}

		//////
		// Write values.
		//////

		var options []option.WriteFunc

		if err := awsssmProvider.Write(context.Background(), valuesToWrite, options...); err != nil {
			log.Fatalln(err)
		}

		log.Printf("Successfully wrote %d parameter(s) to %s", len(valuesToWrite), path)
	},
}

// splitKeyValue splits a KEY=value string into its parts.
func splitKeyValue(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}

	return []string{s}
}

// loadValuesFromFile loads key=value pairs from a file.
func loadValuesFromFile(filename string) (map[string]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string)
	lines := splitLines(string(data))

	for _, line := range lines {
		line = trimSpace(line)

		// Skip empty lines and comments.
		if line == "" || line[0] == '#' {
			continue
		}

		parts := splitKeyValue(line)
		if len(parts) == 2 {
			values[parts[0]] = parts[1]
		}
	}

	return values, nil
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string

	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}

			lines = append(lines, line)
			start = i + 1
		}
	}

	if start < len(s) {
		lines = append(lines, s[start:])
	}

	return lines
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start := 0

	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}

	end := len(s)

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}

	return s[start:end]
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
	awsssmWriteCmd.MarkFlagRequired("path")

	// Values.
	awsssmWriteCmd.Flags().StringSliceP("value", "v", nil, "Key=value pair to write (can be repeated)")
	awsssmWriteCmd.Flags().StringP("file", "f", "", "File containing key=value pairs to write")

	awsssmWriteCmd.SetUsageTemplate(providerUsageTemplate)
}
