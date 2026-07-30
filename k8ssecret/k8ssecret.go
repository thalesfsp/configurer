package k8ssecret

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Name of the provider.
const Name = "k8ssecret"

const (
	serviceAccountDirectory = "/var/run/secrets/kubernetes.io/serviceaccount"
	defaultTokenFile        = serviceAccountDirectory + "/token"
	defaultCACertFile       = serviceAccountDirectory + "/ca.crt"
)

// Config contains Kubernetes Secret authentication and configuration settings.
type Config struct {
	Path                  string `json:"path"                     validate:"omitempty"`
	APIServer             string `json:"api_server"               validate:"omitempty,url"`
	Namespace             string `json:"namespace"                validate:"omitempty,gte=1"`
	SecretName            string `json:"secret_name"              validate:"omitempty,gte=1"`
	Token                 string `json:"-"                        validate:"omitempty"`
	TokenFile             string `json:"token_file"               validate:"omitempty"`
	CACertFile            string `json:"ca_cert_file"             validate:"omitempty"`
	InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify"`
}

// K8sSecret provider definition.
type K8sSecret struct {
	*provider.Provider `json:"-" validate:"required"`

	Configuration *Config      `json:"-" validate:"required"`
	client        *http.Client `json:"-" validate:"required"`
}

type secretResponse struct {
	Data map[string]string `json:"data"`
}

type secretPatch struct {
	Data map[string]string `json:"data"`
}

type secretCreate struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   secretMetadata    `json:"metadata"`
	Data       map[string]string `json:"data"`
}

type secretMetadata struct {
	Name string `json:"name"`
}

//////
// IProvider implementation.
//////

// Load retrieves secrets from a mounted Kubernetes Secret or the Kubernetes API
// and exports them to the environment.
func (k *K8sSecret) Load(
	ctx context.Context,
	opts ...option.LoadKeyFunc,
) (map[string]string, error) {
	if k.Configuration.Path != "" {
		return k.loadMounted(opts)
	}

	return k.loadAPI(ctx, opts)
}

// Write stores secrets through the Kubernetes API.
func (k *K8sSecret) Write(
	ctx context.Context,
	values map[string]interface{},
	opts ...option.WriteFunc,
) error {
	if k.Configuration.Path != "" {
		return provider.ErrNotSupported
	}

	if values == nil {
		return customerror.NewRequiredError("values")
	}

	var options option.Write

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	data := make(map[string]string, len(values))
	for key, value := range values {
		data[key] = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%v", value)))
	}

	payload, err := json.Marshal(&secretPatch{Data: data})
	if err != nil {
		return customerror.NewFailedToError(
			"marshal Kubernetes secret patch",
			customerror.WithError(err),
		)
	}

	response, err := k.request(
		ctx,
		http.MethodPatch,
		k.secretURL(),
		"application/strategic-merge-patch+json",
		payload,
	)
	if err != nil {
		return customerror.NewFailedToError(
			"patch Kubernetes secret",
			customerror.WithError(err),
		)
	}

	if response.StatusCode == http.StatusNotFound {
		_ = response.Body.Close()

		return k.create(ctx, data)
	}
	defer response.Body.Close()

	if !successful(response.StatusCode) {
		return customerror.NewFailedToError(
			"patch Kubernetes secret",
			customerror.WithError(responseStatusError(response)),
		)
	}

	return nil
}

//////
// Helpers.
//////

func (k *K8sSecret) loadMounted(opts []option.LoadKeyFunc) (map[string]string, error) {
	entries, err := os.ReadDir(k.Configuration.Path)
	if err != nil {
		return nil, customerror.NewFailedToError(
			"read mounted Kubernetes secret directory",
			customerror.WithError(err),
		)
	}

	values := make(map[string]string)

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		path := filepath.Join(k.Configuration.Path, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			return nil, customerror.NewFailedToError(
				fmt.Sprintf("stat mounted Kubernetes secret '%s'", entry.Name()),
				customerror.WithError(err),
			)
		}

		if !info.Mode().IsRegular() {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil, customerror.NewFailedToError(
				fmt.Sprintf("read mounted Kubernetes secret '%s'", entry.Name()),
				customerror.WithError(err),
			)
		}

		value := strings.TrimSuffix(string(content), "\n")
		value = strings.TrimSuffix(value, "\r")

		if err := k.exportValue(values, entry.Name(), value, opts); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func (k *K8sSecret) loadAPI(
	ctx context.Context,
	opts []option.LoadKeyFunc,
) (map[string]string, error) {
	response, err := k.request(ctx, http.MethodGet, k.secretURL(), "", nil)
	if err != nil {
		return nil, customerror.NewFailedToError(
			"load Kubernetes secret",
			customerror.WithError(err),
		)
	}
	defer response.Body.Close()

	if !successful(response.StatusCode) {
		return nil, customerror.NewFailedToError(
			"load Kubernetes secret",
			customerror.WithError(responseStatusError(response)),
		)
	}

	var result secretResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, customerror.NewFailedToError(
			"decode Kubernetes secret",
			customerror.WithError(err),
		)
	}

	values := make(map[string]string, len(result.Data))
	for key, encodedValue := range result.Data {
		decodedValue, err := base64.StdEncoding.DecodeString(encodedValue)
		if err != nil {
			return nil, customerror.NewFailedToError(
				fmt.Sprintf("decode Kubernetes secret key '%s'", key),
				customerror.WithError(err),
			)
		}

		if err := k.exportValue(values, key, string(decodedValue), opts); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func (k *K8sSecret) exportValue(
	values map[string]string,
	key string,
	value interface{},
	opts []option.LoadKeyFunc,
) error {
	for _, opt := range opts {
		key = opt(key)
	}

	finalValue, err := provider.ExportToEnvVar(k, key, value)
	if err != nil {
		return err
	}

	values[key] = finalValue

	return nil
}

func (k *K8sSecret) create(ctx context.Context, data map[string]string) error {
	payload, err := json.Marshal(&secretCreate{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata: secretMetadata{
			Name: k.Configuration.SecretName,
		},
		Data: data,
	})
	if err != nil {
		return customerror.NewFailedToError(
			"marshal Kubernetes secret",
			customerror.WithError(err),
		)
	}

	response, err := k.request(
		ctx,
		http.MethodPost,
		k.secretsURL(),
		"application/json",
		payload,
	)
	if err != nil {
		return customerror.NewFailedToError(
			"create Kubernetes secret",
			customerror.WithError(err),
		)
	}
	defer response.Body.Close()

	if !successful(response.StatusCode) {
		return customerror.NewFailedToError(
			"create Kubernetes secret",
			customerror.WithError(responseStatusError(response)),
		)
	}

	return nil
}

func (k *K8sSecret) request(
	ctx context.Context,
	method, endpoint, contentType string,
	body []byte,
) (*http.Response, error) {
	token, err := k.bearerToken()
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	return k.client.Do(request)
}

func (k *K8sSecret) bearerToken() (string, error) {
	if k.Configuration.Token != "" {
		return k.Configuration.Token, nil
	}

	token, err := os.ReadFile(k.Configuration.TokenFile)
	if err != nil {
		return "", customerror.NewFailedToError(
			"read Kubernetes token file",
			customerror.WithError(err),
		)
	}

	value := strings.TrimSpace(string(token))
	if value == "" {
		return "", customerror.NewInvalidError("Kubernetes token file is empty")
	}

	return value, nil
}

func (k *K8sSecret) secretsURL() string {
	return fmt.Sprintf(
		"%s/api/v1/namespaces/%s/secrets",
		strings.TrimRight(k.Configuration.APIServer, "/"),
		url.PathEscape(k.Configuration.Namespace),
	)
}

func (k *K8sSecret) secretURL() string {
	return fmt.Sprintf(
		"%s/%s",
		k.secretsURL(),
		url.PathEscape(k.Configuration.SecretName),
	)
}

func successful(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

func responseStatusError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("Kubernetes API returned %s", response.Status)
	}

	return fmt.Errorf("Kubernetes API returned %s: %s", response.Status, message)
}

//////
// Factory.
//////

// New creates a Kubernetes Secrets provider.
func New(override, rawValue bool, config *Config) (provider.IProvider, error) {
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}

	applyDefaults(config)

	if err := validation.Validate(config); err != nil {
		return nil, err
	}

	if config.Path == "" {
		if config.APIServer == "" {
			return nil, customerror.NewRequiredError("path or API server")
		}
		if config.SecretName == "" {
			return nil, customerror.NewRequiredError("secret name")
		}
		if config.Token == "" && config.TokenFile == "" {
			return nil, customerror.NewRequiredError("token or token file")
		}
	}

	baseProvider, err := provider.New(Name, override, rawValue)
	if err != nil {
		return nil, err
	}

	client, err := newHTTPClient(config)
	if err != nil {
		return nil, err
	}

	kubernetesSecret := &K8sSecret{
		Provider:      baseProvider,
		Configuration: config,
		client:        client,
	}

	if err := validation.Validate(kubernetesSecret); err != nil {
		return nil, err
	}

	return kubernetesSecret, nil
}

func applyDefaults(config *Config) {
	if config.Namespace == "" {
		config.Namespace = "default"
	}

	if config.APIServer == "" {
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		if host != "" {
			port := os.Getenv("KUBERNETES_SERVICE_PORT")
			if port == "" {
				port = "443"
			}

			config.APIServer = "https://" + net.JoinHostPort(host, port)
		}
	}

	if config.TokenFile == "" && regularFileExists(defaultTokenFile) {
		config.TokenFile = defaultTokenFile
	}

	if config.CACertFile == "" && regularFileExists(defaultCACertFile) {
		config.CACertFile = defaultCACertFile
	}
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

func newHTTPClient(config *Config) (*http.Client, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{}
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: config.InsecureSkipTLSVerify, //nolint:gosec // Explicit user configuration.
	}

	if config.CACertFile != "" {
		certificate, err := os.ReadFile(config.CACertFile)
		if err != nil {
			return nil, customerror.NewFailedToError(
				"read Kubernetes CA certificate",
				customerror.WithError(err),
			)
		}

		certificates, err := x509.SystemCertPool()
		if err != nil {
			return nil, customerror.NewFailedToError(
				"load system CA certificates",
				customerror.WithError(err),
			)
		}

		if !certificates.AppendCertsFromPEM(certificate) {
			return nil, customerror.NewInvalidError("Kubernetes CA certificate is invalid")
		}

		tlsConfig.RootCAs = certificates
	}

	transport.TLSClientConfig = tlsConfig

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}
