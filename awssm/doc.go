// Package awssm provides an AWS Secrets Manager provider for the configurer application.
//
// The AWS Secrets Manager provider allows you to load secrets from AWS Secrets Manager
// and optionally write configuration data to AWS Secrets Manager. It supports both JSON
// and plain text secrets with various authentication methods.
//
// # Features
//
// - Load secrets from AWS Secrets Manager and export them as environment variables
// - Write configuration data to AWS Secrets Manager as JSON secrets
// - Support for both JSON and plain text secret formats
// - Multiple authentication methods (IAM roles, profiles, access keys)
// - Key transformation options (prefixing, suffixing, case conversion)
//
// # Authentication Methods
//
// The provider supports the following authentication methods:
//   - Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
//   - AWS credentials file (~/.aws/credentials)
//   - IAM roles (recommended for EC2/ECS/Lambda)
//   - AWS SSO
//
// # Environment Variables
//
// The following environment variables can be used to configure the provider:
//   - AWS_REGION: The AWS region where secrets are stored
//   - AWS_PROFILE: The AWS profile to use for authentication
//   - AWS_ACCESS_KEY_ID: The AWS access key ID (not recommended, use IAM roles instead)
//   - AWS_SECRET_ACCESS_KEY: The AWS secret access key (not recommended, use IAM roles instead)
//   - AWSSM_SECRET_NAME: The secret name to load from or write to AWS Secrets Manager
//
// # Usage Examples
//
// Loading secrets:
//
//	configurer l awssm -r us-east-1 -s myapp/prod/secrets -- env
//
// Writing configuration:
//
//	configurer w --source dev.env awssm -r us-east-1 -n myapp/prod/secrets
//
// # Secret Formats
//
// For JSON secrets, each key-value pair will be exported as a separate environment variable.
// For plain text secrets, the secret name (last part after /) will be used as the
// environment variable name.
//
// When writing secrets, all key-value pairs from the source file will be stored as a
// single JSON object in the specified secret.
//
// # Security Notes
//
// - It's recommended to use IAM roles for authentication instead of access keys
// - Already exported environment variables have precedence over loaded ones
// - Use the override flag to override existing environment variables
package awssm
