package awsssm

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
)

// Name of the provider.
const Name = "awsssm"

// Config contains AWS configuration settings.
type Config struct {
	Region    string `json:"region"     validate:"omitempty,gte=1"`
	Profile   string `json:"profile"    validate:"omitempty,gte=1"`
	AccessKey string `json:"access_key" validate:"omitempty,gte=1"`
	SecretKey string `json:"secret_key" validate:"omitempty,gte=1"`
}

// ParameterInformation contains information about which parameters to retrieve.
type ParameterInformation struct {
	// ParameterNames is a list of specific parameter names to retrieve.
	ParameterNames []string `json:"parameter_names" validate:"omitempty"`

	// Path is a parameter path prefix. If set, all parameters under this path
	// will be retrieved recursively.
	Path string `json:"path" validate:"omitempty"`

	// Recursive determines whether to recursively retrieve all parameters under Path.
	// Only applicable when Path is set.
	Recursive bool `json:"recursive"`

	// WithDecryption determines whether to decrypt SecureString parameters.
	// Defaults to true.
	WithDecryption bool `json:"with_decryption"`
}

// AWSSSM provider definition.
type AWSSSM struct {
	*provider.Provider    `json:"-" validate:"required"`
	*Config               `json:"-" validate:"required"`
	*ParameterInformation `json:"-" validate:"required"`

	client *ssm.Client `json:"-" validate:"required"`
}

// extractKeyFromPath extracts the parameter name from a full path.
// For example, "/myapp/prod/DB_HOST" -> "DB_HOST".
func extractKeyFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}

	return parts[len(parts)-1]
}

// Load retrieves parameters from AWS SSM Parameter Store and exports them to the environment.
func (a *AWSSSM) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
	finalValues := make(map[string]string)

	// If Path is specified, get all parameters under that path.
	if a.ParameterInformation.Path != "" {
		if err := a.loadByPath(ctx, finalValues, opts); err != nil {
			return nil, err
		}
	}

	// If specific parameter names are specified, get those.
	if len(a.ParameterInformation.ParameterNames) > 0 {
		if err := a.loadByNames(ctx, finalValues, opts); err != nil {
			return nil, err
		}
	}

	return finalValues, nil
}

// loadByPath retrieves all parameters under a given path prefix.
func (a *AWSSSM) loadByPath(ctx context.Context, finalValues map[string]string, opts []option.LoadKeyFunc) error {
	input := &ssm.GetParametersByPathInput{
		Path:           aws.String(a.ParameterInformation.Path),
		Recursive:      aws.Bool(a.ParameterInformation.Recursive),
		WithDecryption: aws.Bool(a.ParameterInformation.WithDecryption),
	}

	paginator := ssm.NewGetParametersByPathPaginator(a.client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return customerror.NewFailedToError(
				fmt.Sprintf("get parameters by path '%s'", a.ParameterInformation.Path),
				customerror.WithError(err),
			)
		}

		for _, param := range output.Parameters {
			if param.Name == nil || param.Value == nil {
				continue
			}

			key := extractKeyFromPath(*param.Name)

			// Apply key transformation options.
			for _, opt := range opts {
				key = opt(key)
			}

			value := *param.Value

			// Handle StringList type (comma-separated values).
			if param.Type == types.ParameterTypeStringList {
				// StringList values are already comma-separated in the Value field.
				value = *param.Value
			}

			finalValue, err := provider.ExportToEnvVar(a, key, value)
			if err != nil {
				return err
			}

			finalValues[key] = finalValue
		}
	}

	return nil
}

// loadByNames retrieves specific parameters by their names.
func (a *AWSSSM) loadByNames(ctx context.Context, finalValues map[string]string, opts []option.LoadKeyFunc) error {
	// SSM GetParameters can handle up to 10 parameters at a time.
	const batchSize = 10

	names := a.ParameterInformation.ParameterNames

	for i := 0; i < len(names); i += batchSize {
		end := min(i+batchSize, len(names))

		batch := names[i:end]

		input := &ssm.GetParametersInput{
			Names:          batch,
			WithDecryption: aws.Bool(a.ParameterInformation.WithDecryption),
		}

		output, err := a.client.GetParameters(ctx, input)
		if err != nil {
			return customerror.NewFailedToError(
				"get parameters",
				customerror.WithError(err),
			)
		}

		// Check for invalid parameters.
		if len(output.InvalidParameters) > 0 {
			return customerror.NewNotFoundError(
				fmt.Sprintf("parameters: %s", strings.Join(output.InvalidParameters, ", ")),
			)
		}

		for _, param := range output.Parameters {
			if param.Name == nil || param.Value == nil {
				continue
			}

			key := extractKeyFromPath(*param.Name)

			// Apply key transformation options.
			for _, opt := range opts {
				key = opt(key)
			}

			value := *param.Value

			finalValue, err := provider.ExportToEnvVar(a, key, value)
			if err != nil {
				return err
			}

			finalValues[key] = finalValue
		}
	}

	return nil
}

// Write stores parameters in AWS SSM Parameter Store.
func (a *AWSSSM) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
	if values == nil {
		return customerror.NewRequiredError("values")
	}

	// Process the options.
	var options option.Write

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	// Determine the base path for writing parameters.
	basePath := a.ParameterInformation.Path
	if basePath == "" && len(a.ParameterInformation.ParameterNames) > 0 {
		// Use the directory of the first parameter name as base path.
		parts := strings.Split(a.ParameterInformation.ParameterNames[0], "/")
		if len(parts) > 1 {
			basePath = strings.Join(parts[:len(parts)-1], "/")
		}
	}

	if basePath == "" {
		basePath = "/"
	}

	// Ensure basePath starts with / and doesn't end with /.
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	basePath = strings.TrimSuffix(basePath, "/")

	// Write each value as a parameter.
	for key, value := range values {
		paramName := fmt.Sprintf("%s/%s", basePath, key)

		// Convert value to string.
		var strValue string

		switch v := value.(type) {
		case string:
			strValue = v
		case []string:
			strValue = strings.Join(v, ",")
		default:
			strValue = fmt.Sprintf("%v", v)
		}

		input := &ssm.PutParameterInput{
			Name:      aws.String(paramName),
			Value:     aws.String(strValue),
			Type:      types.ParameterTypeSecureString, // Default to SecureString for security.
			Overwrite: aws.Bool(true),
		}

		_, err := a.client.PutParameter(ctx, input)
		if err != nil {
			return customerror.NewFailedToError(
				fmt.Sprintf("put parameter '%s'", paramName),
				customerror.WithError(err),
			)
		}
	}

	return nil
}

// NewWithConfig creates a new AWS SSM Parameter Store provider with custom AWS configuration.
func NewWithConfig(
	override, rawValue bool,
	config *Config,
	paramInfo *ParameterInformation,
	awsConfig aws.Config,
) (provider.IProvider, error) {
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}

	if paramInfo == nil {
		return nil, customerror.NewRequiredError("parameter information")
	}

	// Validate that at least one of Path or ParameterNames is specified.
	if paramInfo.Path == "" && len(paramInfo.ParameterNames) == 0 {
		return nil, customerror.NewRequiredError("either path or parameter_names")
	}

	p, err := provider.New(Name, override, rawValue)
	if err != nil {
		return nil, err
	}

	awsssm := &AWSSSM{
		Provider:             p,
		Config:               config,
		ParameterInformation: paramInfo,
	}

	if err := validation.Validate(awsssm); err != nil {
		return nil, err
	}

	// Create AWS SSM client.
	awsssm.client = ssm.NewFromConfig(awsConfig)

	return awsssm, nil
}

// New creates a new AWS SSM Parameter Store provider.
//
// The following environment variables can be used to configure the provider:
//
//   - AWS_REGION: The AWS region where parameters are stored.
//   - AWS_PROFILE: The AWS profile to use for authentication.
//   - AWS_ACCESS_KEY_ID: The AWS access key ID (not recommended, use IAM roles instead).
//   - AWS_SECRET_ACCESS_KEY: The AWS secret access key (not recommended, use IAM roles instead).
//   - AWSSSM_PATH: The parameter path prefix to load.
//   - AWSSSM_PARAMETER_NAME: A specific parameter name to load.
//
// NOTE: It's recommended to use IAM roles for authentication instead of access keys.
func New(
	override, rawValue bool,
	config *Config,
	paramInfo *ParameterInformation,
) (provider.IProvider, error) {
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}

	if paramInfo == nil {
		return nil, customerror.NewRequiredError("parameter information")
	}

	// Load AWS configuration.
	var (
		awsConfig aws.Config
		err       error
	)

	if config.Profile != "" {
		awsConfig, err = awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(config.Region),
			awsconfig.WithSharedConfigProfile(config.Profile),
		)
	} else {
		awsConfig, err = awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(config.Region),
		)
	}

	if err != nil {
		return nil, customerror.NewFailedToError("load AWS config", customerror.WithError(err))
	}

	// If access keys are provided.
	if config.AccessKey != "" && config.SecretKey != "" {
		awsConfig.Credentials = aws.CredentialsProviderFunc(func(_ context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     config.AccessKey,
				SecretAccessKey: config.SecretKey,
			}, nil
		})
	}

	return NewWithConfig(override, rawValue, config, paramInfo, awsConfig)
}
