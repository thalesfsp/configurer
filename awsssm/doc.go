// Package awsssm provides an AWS Systems Manager Parameter Store provider.
//
// AWS SSM Parameter Store is a hierarchical configuration store. It's commonly
// used for storing configuration parameters, secrets, and other data that
// applications need at runtime.
//
// # Key Differences from AWS Secrets Manager (awssm)
//
//   - Hierarchical paths: Parameters are organized in paths like /myapp/prod/DB_HOST
//   - Parameter types: String, StringList, and SecureString (encrypted)
//   - Free tier: Standard parameters are free, making it cost-effective
//   - Path-based retrieval: Can fetch all parameters under a path prefix
//
// # Setting up the environment
//
// Flags (not recommended):
//
//	configurer l awsssm \
//	  --region         "us-east-1" \
//	  --path           "/myapp/prod" \
//	  --profile        "default" -- env
//
// Or for a single parameter:
//
//	configurer l awsssm \
//	  --region         "us-east-1" \
//	  --parameter-name "/myapp/prod/DB_HOST" \
//	  --profile        "default" -- env
//
// Environment Variables (recommended):
//
//	export AWS_REGION="us-east-1"
//	export AWS_PROFILE="default"
//	export AWSSSM_PATH="/myapp/prod"
//
//	configurer l awsssm -- env
package awsssm
