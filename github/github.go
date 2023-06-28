// Package github provides a GitHub provider.
package github

import (
	"context"
	"encoding/base64"
	"fmt"
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
		encryptedValue, err := encrypt(v.PublicKeyResponse.Key, fmt.Sprintf("%v", item))
		if err != nil {
			return false, err
		}

		secretsRequest := &SecretRequest{
			EncryptedValue: encryptedValue,
			KeyID:          v.PublicKeyResponse.KeyID,
		}

		if repository != nil {
			if _, err := v.client.Put(
				ctx,
				fmt.Sprintf(
					"https://api.github.com/repositories/%d/environments/%s/secrets/%s",
					repository.ID,
					options.Environment,
					key,
				),
				httpclient.WithReqBody(secretsRequest),
			); err != nil {
				return false, err
			}

			return false, nil
		}

		if _, err := v.client.Put(
			ctx,
			fmt.Sprintf(
				"https://api.github.com/repos/%s/%s/actions/secrets/%s",
				v.Owner,
				v.Repo,
				key,
			),
			httpclient.WithReqBody(secretsRequest),
		); err != nil {
			return false, err
		}

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
) (provider.IProvider, error) {
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
