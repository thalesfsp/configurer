// Package github provides a GitHub provider.
package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/httpclient"
	"github.com/thalesfsp/validation"
	"golang.org/x/crypto/nacl/box"
)

//////
// Vars, consts, and types.
//////

// Name of the provider.
const Name = "github"

// Config is an alias to GitHub configuration.
// type Config = *github.Config

// SecretInformation is the information about a secret, where to retrieve it.
type SecretInformation struct {
	MountPath  string `json:"-" validate:"required"`
	SecretPath string `json:"-" validate:"required"`
}

// PublicKeyResponse is the response from the GitHub API.
type PublicKeyResponse struct {
	Key   string `json:"key"`
	KeyID string `json:"key_id"`
}

// VariableRequest is the request to store a new secret.
type VariableRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SecretRequest is the request to store a new secret.
type SecretRequest struct {
	EncryptedValue string `json:"encrypted_value"`
	KeyID          string `json:"key_id"`
}

// Repository is the repository information.
type Repository struct {
	ID     int    `json:"id"`
	NodeID string `json:"node_id"`
}

// GitHub provider definition.
type GitHub struct {
	*provider.Provider `json:"-" validate:"required"`

	*PublicKeyResponse `json:"-" validate:"required"`

	Owner string `json:"owner" validate:"required"`
	Repo  string `json:"repo" validate:"required"`
	Token string `json:"-" validate:"required"`

	client *httpclient.Client `json:"-" validate:"required"`
}

// SecretsResponseSecret is the secret information from the GitHub API.
type SecretsResponseSecret struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SecretsResponse is the response from listing secrets.
type SecretsResponse struct {
	TotalCount int                     `json:"total_count"`
	Secrets    []SecretsResponseSecret `json:"secrets"`
}

//////
// Helpers.
//////

// encrypt encrypts the secret using the public key.
func encrypt(publicKey, secret string) (string, error) {
	// Decode the public key
	decodedPublicKey, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return "", customerror.NewFailedToError("decode public key", customerror.WithError(err))
	}

	// Convert the decoded public key to [32]byte.
	var publicKeyBytes [32]byte

	// Copy the decoded public key to the publicKeyBytes.
	copy(publicKeyBytes[:], decodedPublicKey[:32])

	// Encrypt the secret.
	encrypted, err := box.SealAnonymous(nil, []byte(secret), &publicKeyBytes, nil)
	if err != nil {
		return "", customerror.NewFailedToError("encrypt secret", customerror.WithError(err))
	}

	// Encode the encrypted secret to base64
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// GetRepository retrieves the repository information.
func (v *GitHub) GetRepository(ctx context.Context) (*Repository, error) {
	var repository Repository

	r, err := v.client.Get(
		ctx,
		fmt.Sprintf("https://api.github.com/repos/%s/%s", v.Owner, v.Repo),
		httpclient.WithRespBody(&repository),
	)
	if err != nil {
		return nil, err
	}

	defer r.Body.Close()

	return &repository, nil
}

// List secrets.
func List(ctx context.Context, v *GitHub) (*SecretsResponse, error) {
	var sR SecretsResponse

	resp, err := v.client.Get(
		ctx,
		fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/actions/secrets",
			v.Owner,
			v.Repo,
		),
		httpclient.WithRespBody(&sR),
	)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return &sR, nil
}

// Delete secrets.
func Delete(ctx context.Context, v *GitHub, secrets ...string) error {
	if _, err := concurrentloop.Map(ctx, secrets, func(ctx context.Context, secret string) (bool, error) {
		resp, err := v.client.Delete(
			ctx,
			fmt.Sprintf(
				"https://api.github.com/repos/%s/%s/actions/secrets/%s",
				v.Owner,
				v.Repo,
				secret,
			),
		)
		if err != nil {
			return false, err
		}

		defer resp.Body.Close()

		return true, nil
	},
		concurrentloop.WithBatchSize(10),
		concurrentloop.WithRandomDelayTime(100, 700, time.Millisecond),
	); err != nil {
		return err
	}

	return nil
}

//////
// IProvider implementation.
//////

// Load retrieves the configuration, and exports it to the environment.
//
// NOTE: Not all providers allow loading secrets, for example, GitHub. They are
// designed to be write-only stores of information. This is a security measure
// to prevent exposure of sensitive data.
func (v *GitHub) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
	return nil, provider.ErrNotSupported
}

// Write stores a new secret.
//
// NOTE: Not all providers support writing secrets.
func (v *GitHub) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
	// Ensure the secret values are not nil.
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

	var repository *Repository

	if options.Environment != "" {
		repo, err := v.GetRepository(ctx)
		if err != nil {
			return err
		}

		repository = repo
	}

	// Write the secrets.
	if _, err := concurrentloop.MapM(ctx, values, func(ctx context.Context, key string, item any) (bool, error) {
		variableRequest := &VariableRequest{
			Name:  key,
			Value: fmt.Sprintf("%v", item),
		}

		encryptedValue, err := encrypt(v.PublicKeyResponse.Key, fmt.Sprintf("%v", item))
		if err != nil {
			return false, err
		}

		secretRequest := &SecretRequest{
			EncryptedValue: encryptedValue,
			KeyID:          v.PublicKeyResponse.KeyID,
		}

		//////
		// Default case: it's a REPOSITORY AND A SECRET.
		//////

		finalVerb := http.MethodPut

		finalURL := fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/actions/secrets/%s",
			v.Owner,
			v.Repo,
			key,
		)

		finalReqBody := httpclient.WithReqBody(secretRequest)

		//////
		// Deal with cases where it's a REPOSITORY AND a VARIABLE.
		//////

		if options.Variable {
			finalURL = fmt.Sprintf(
				"https://api.github.com/repos/%s/%s/actions/variables",
				v.Owner,
				v.Repo,
			)

			finalReqBody = httpclient.WithReqBody(variableRequest)

			finalVerb = http.MethodPost
		}

		//////
		// Deal with cases where it's an ENVIRONMENT.
		//////

		if repository != nil {
			finalURL = fmt.Sprintf(
				"https://api.github.com/repositories/%d/environments/%s/secrets/%s",
				repository.ID,
				options.Environment,
				key,
			)

			finalReqBody = httpclient.WithReqBody(secretRequest)

			finalVerb = http.MethodPut

			//////
			// Deal with cases where it's an ENVIRONMENT AND a VARIABLE.
			//////

			if options.Variable {
				finalURL = fmt.Sprintf(
					"https://api.github.com/repositories/%d/environments/%s/variables",
					repository.ID,
					options.Environment,
				)

				finalReqBody = httpclient.WithReqBody(variableRequest)

				finalVerb = http.MethodPost
			}
		}

		if finalVerb == http.MethodPost {
			resp, err := v.client.Post(ctx, finalURL, finalReqBody)
			if err != nil {
				return false, err
			}

			defer resp.Body.Close()

			return true, nil
		}

		resp, err := v.client.Put(
			ctx,
			finalURL,
			finalReqBody,
		)
		if err != nil {
			return false, err
		}

		defer resp.Body.Close()

		return true, nil
	},
		concurrentloop.WithBatchSize(10),
		concurrentloop.WithRandomDelayTime(100, 700, time.Millisecond),
	); err != nil {
		return err
	}

	return nil
}

//////
// Factory.
//////

// New returns a new GitHub provider.
func New(
	override bool,
	owner, repo string,
) (*GitHub, error) {
	provider, err := provider.New(Name, override)
	if err != nil {
		return nil, err
	}

	token := os.Getenv("GITHUB_TOKEN")

	if token == "" {
		return nil, customerror.NewRequiredError("GITHUB_TOKEN env var")
	}

	client, err := httpclient.NewDefault(Name)
	if err != nil {
		return nil, err
	}

	// Setup default headers.
	client.Headers = map[string]string{
		"Accept":        "application/vnd.github+json",
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}

	// Retrieve the public key.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var publicKeyResponse PublicKeyResponse

	r, err := client.Get(
		ctx,
		fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/secrets/public-key", owner, repo),
		httpclient.WithRespBody(&publicKeyResponse),
	)
	if err != nil {
		return nil, customerror.NewRequiredError("publicKey information")
	}

	defer r.Body.Close()

	v := &GitHub{
		Provider:          provider,
		PublicKeyResponse: &publicKeyResponse,

		Owner: owner,
		Repo:  repo,
		Token: token,

		client: client,
	}

	if err := validation.Validate(v); err != nil {
		return nil, err
	}

	return v, nil
}
