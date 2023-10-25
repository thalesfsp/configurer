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

// Target of the request.
type Target string

const (
	// Actions const.
	Actions Target = "actions"

	// Codespaces const.
	Codespaces Target = "codespaces"
)

// String implements the Stringer interface.
func (t Target) String() string {
	return string(t)
}

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

	publicKeyResponseActions   *PublicKeyResponse `json:"-" validate:"required"`
	publicKeyResponseCodespace *PublicKeyResponse `json:"-" validate:"required"`

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

// RetrieveKey retrieves the public key to sign the data.
func retrieveKey(
	ctx context.Context,
	c *httpclient.Client,
	owner, repo string,
	target Target,
) (*PublicKeyResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var publicKeyResponse PublicKeyResponse

	r, err := c.Get(
		ctx,
		fmt.Sprintf("https://api.github.com/repos/%s/%s/%s/secrets/public-key", owner, repo, target),
		httpclient.WithRespBody(&publicKeyResponse),
	)
	if err != nil {
		return nil, customerror.NewRequiredError("publicKey information")
	}

	defer r.Body.Close()

	return &publicKeyResponse, nil
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
	if values == nil {
		return customerror.NewRequiredError("values")
	}

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

	_, err := concurrentloop.MapM(ctx, values, func(ctx context.Context, key string, item any) (bool, error) {
		variableRequest := &VariableRequest{
			Name:  key,
			Value: fmt.Sprintf("%v", item),
		}

		encryptedValue, err := encrypt(v.publicKeyResponseCodespace.Key, fmt.Sprintf("%v", item))
		if err != nil {
			return false, err
		}

		secretRequest := &SecretRequest{
			EncryptedValue: encryptedValue,
			KeyID:          v.publicKeyResponseCodespace.KeyID,
		}

		finalVerb, finalURL, finalReqBody := v.constructRequestDetails(
			options,
			repository,
			key,
			variableRequest,
			secretRequest,
			options.HTTPVerb,
		)

		if finalVerb == http.MethodPost {
			return v.executePOSTRequest(ctx, finalURL, finalReqBody)
		}

		if finalVerb == http.MethodPatch {
			return v.executePATCHRequest(ctx, finalURL, finalReqBody)
		}

		return v.executePUTRequest(ctx, finalURL, finalReqBody)
	},
		concurrentloop.WithBatchSize(10),
		concurrentloop.WithRandomDelayTime(100, 700, time.Millisecond),
	)

	return err
}

func (v *GitHub) constructRequestDetails(
	options option.Write,
	repository *Repository,
	key string,
	variableRequest *VariableRequest,
	secretRequest *SecretRequest,
	forceVerb string,
) (string, string, httpclient.Func) {
	finalVerb := http.MethodPut
	finalURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/%s/secrets/%s",
		v.Owner,
		v.Repo,
		options.Target,
		key,
	)
	finalReqBody := httpclient.WithReqBody(secretRequest)

	if options.Variable {
		finalURL = fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/%s/variables",
			v.Owner,
			v.Repo,
			options.Target,
		)

		finalReqBody = httpclient.WithReqBody(variableRequest)
		finalVerb = http.MethodPost
	}

	if repository != nil {
		finalURL = fmt.Sprintf(
			"https://api.github.com/repositories/%d/environments/%s/secrets/%s",
			repository.ID,
			options.Environment,
			key,
		)
		finalReqBody = httpclient.WithReqBody(secretRequest)
		finalVerb = http.MethodPut

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

	if forceVerb != "" {
		if options.Variable {
			finalURL = fmt.Sprintf(
				"https://api.github.com/repositories/%d/environments/%s/variables/%s",
				repository.ID,
				options.Environment,
				key,
			)

			finalReqBody = httpclient.WithReqBody(variableRequest)
		}

		finalVerb = forceVerb
	}

	return finalVerb, finalURL, finalReqBody
}

func (v *GitHub) executePOSTRequest(
	ctx context.Context,
	url string,
	reqBody httpclient.Func,
) (bool, error) {
	resp, err := v.client.Post(ctx, url, reqBody)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	return true, nil
}

func (v *GitHub) executePUTRequest(
	ctx context.Context,
	url string,
	reqBody httpclient.Func,
) (bool, error) {
	resp, err := v.client.Put(ctx, url, reqBody)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	return true, nil
}

func (v *GitHub) executePATCHRequest(
	ctx context.Context,
	url string,
	reqBody httpclient.Func,
) (bool, error) {
	resp, err := v.client.Patch(ctx, url, reqBody)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	return true, nil
}

//////
// Factory.
//////

// New returns a new GitHub provider.
func New(
	override, rawValue bool,
	owner, repo string,
) (*GitHub, error) {
	provider, err := provider.New(Name, override, rawValue)
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
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
		"Authorization":        fmt.Sprintf("Bearer %s", token),
	}

	// Retrieve the public key.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	publicKeyResponseActions, err := retrieveKey(ctx, client, owner, repo, Actions)
	if err != nil {
		return nil, err
	}

	publicKeyResponseCodespace, err := retrieveKey(ctx, client, owner, repo, Codespaces)
	if err != nil {
		return nil, err
	}

	v := &GitHub{
		Provider:                   provider,
		publicKeyResponseActions:   publicKeyResponseActions,
		publicKeyResponseCodespace: publicKeyResponseCodespace,

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
