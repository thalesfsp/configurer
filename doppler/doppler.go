package doppler

import (
	"context"
	"fmt"
	"strings"

	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/httpclient/v2"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Name of the provider.
const Name = "doppler"

var dopplerAPIBaseURL = "https://api.doppler.com"

// Config contains Doppler authentication and configuration settings.
type Config struct {
	Token   string `json:"-"       validate:"required"`
	Project string `json:"project" validate:"omitempty,gte=1"`
	Config  string `json:"config"  validate:"omitempty,gte=1"`
}

// Doppler provider definition.
type Doppler struct {
	*provider.Provider `json:"-" validate:"required"`

	Configuration *Config            `json:"-" validate:"required"`
	client        *httpclient.Client `json:"-" validate:"required"`
}

type writeRequest struct {
	Project string                 `json:"project"`
	Config  string                 `json:"config"`
	Secrets map[string]interface{} `json:"secrets"`
}

//////
// IProvider implementation.
//////

// Load retrieves secrets from Doppler and exports them to the environment.
func (d *Doppler) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
	secrets := make(map[string]interface{})

	resp, err := d.client.Get(
		ctx,
		fmt.Sprintf("%s/v3/configs/config/secrets/download", dopplerAPIBaseURL),
		httpclient.WithQueryParam("format", "json"),
		httpclient.WithQueryParam("project", d.Configuration.Project),
		httpclient.WithQueryParam("config", d.Configuration.Config),
		httpclient.WithRespBody(&secrets),
	)
	if err != nil {
		return nil, customerror.NewFailedToError("download secrets", customerror.WithError(err))
	}

	defer resp.Body.Close()

	if secrets == nil {
		return nil, customerror.NewInvalidError("Doppler secrets payload must be a JSON object")
	}

	finalValues := make(map[string]string, len(secrets))

	for key, value := range secrets {
		for _, opt := range opts {
			key = opt(key)
		}

		finalValue, err := provider.ExportToEnvVar(d, key, value)
		if err != nil {
			return nil, err
		}

		finalValues[key] = finalValue
	}

	return finalValues, nil
}

// Write stores secrets in Doppler.
func (d *Doppler) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
	if values == nil {
		return customerror.NewRequiredError("values")
	}

	var options option.Write

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	resp, err := d.client.Post(
		ctx,
		fmt.Sprintf("%s/v3/configs/config/secrets", dopplerAPIBaseURL),
		httpclient.WithReqBody(&writeRequest{
			Project: d.Configuration.Project,
			Config:  d.Configuration.Config,
			Secrets: values,
		}),
	)
	if err != nil {
		return customerror.NewFailedToError("write secrets", customerror.WithError(err))
	}

	defer resp.Body.Close()

	return nil
}

//////
// Factory.
//////

// New creates a Doppler provider.
//
// Project and Config are optional for Doppler service tokens, which carry
// their own project and config scope. Other token types require both fields.
func New(override, rawValue bool, config *Config) (provider.IProvider, error) {
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}

	if err := validation.Validate(config); err != nil {
		return nil, err
	}

	if !isServiceToken(config.Token) && (config.Project == "" || config.Config == "") {
		return nil, customerror.NewRequiredError("project and config for non-service tokens")
	}

	baseProvider, err := provider.New(Name, override, rawValue)
	if err != nil {
		return nil, err
	}

	client, err := httpclient.NewDefault(
		httpclient.WithClientName(Name),
		httpclient.WithClientHeader("Authorization", fmt.Sprintf("Bearer %s", config.Token)),
	)
	if err != nil {
		return nil, customerror.NewFailedToError("initialize Doppler client", customerror.WithError(err))
	}

	doppler := &Doppler{
		Provider:      baseProvider,
		Configuration: config,
		client:        client,
	}

	if err := validation.Validate(doppler); err != nil {
		return nil, err
	}

	return doppler, nil
}

func isServiceToken(token string) bool {
	return strings.HasPrefix(token, "dp.st.")
}
