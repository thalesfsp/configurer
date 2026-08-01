package vault

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	api "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/internal/testenv"
	"github.com/thalesfsp/configurer/option"
)

//////
// Test helpers.
//////

type recordedVaultRequest struct {
	body   map[string]interface{}
	header http.Header
	method string
	path   string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type vaultTestServer struct {
	address string
	client  *http.Client
	close   func()
}

func cleanVaultEnvironment(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		api.EnvVaultAddress,
		api.EnvVaultAgentAddr,
		api.EnvVaultCACert,
		api.EnvVaultCACertBytes,
		api.EnvVaultCAPath,
		api.EnvVaultClientCert,
		api.EnvVaultClientKey,
		api.EnvVaultClientTimeout,
		api.EnvVaultHeaders,
		api.EnvVaultSRVLookup,
		api.EnvVaultSkipVerify,
		api.EnvVaultNamespace,
		api.EnvVaultTLSServerName,
		api.EnvVaultMaxRetries,
		api.EnvVaultToken,
		api.EnvRateLimit,
		api.EnvHTTPProxy,
		api.EnvVaultProxyAddr,
		api.EnvVaultDisableRedirects,
	} {
		testenv.Unset(t, key)
	}
}

func newVaultTestConfig(t *testing.T, address string, clients ...*http.Client) Config {
	t.Helper()

	config := api.DefaultConfig()
	require.NoError(t, config.Error)

	config.Address = address
	config.MaxRetries = 0

	if len(clients) > 0 {
		config.HttpClient = clients[0]
	}

	return config
}

func newVaultTestServer(
	t *testing.T,
	statusCode int,
	responseBody string,
) (*vaultTestServer, <-chan recordedVaultRequest, *atomic.Int32) {
	t.Helper()

	requests := make(chan recordedVaultRequest, 4)
	var callCount atomic.Int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)

		body := make(map[string]interface{})
		if r.Body != nil {
			rawBody, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
			} else if len(rawBody) > 0 {
				if err := json.Unmarshal(rawBody, &body); err != nil {
					t.Errorf("decode request body: %v", err)
				}
			}
		}

		requests <- recordedVaultRequest{
			body:   body,
			header: r.Header.Clone(),
			method: r.Method,
			path:   r.URL.Path,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if responseBody != "" {
			if _, err := w.Write([]byte(responseBody)); err != nil {
				t.Errorf("write response body: %v", err)
			}
		}
	})

	if server := tryNewVaultHTTPTestServer(handler); server != nil {
		testServer := &vaultTestServer{
			address: server.URL,
			client:  server.Client(),
			close:   server.Close,
		}
		t.Cleanup(testServer.close)

		return testServer, requests, &callCount
	}

	testServer := &vaultTestServer{
		address: "http://vault.test",
		client: &http.Client{
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				response := recorder.Result()
				response.Request = request

				return response, nil
			}),
		},
		close: func() {},
	}
	t.Cleanup(testServer.close)

	return testServer, requests, &callCount
}

func tryNewVaultHTTPTestServer(handler http.Handler) *httptest.Server {
	var testServer *httptest.Server

	func() {
		defer func() {
			if recover() != nil {
				testServer = nil
			}
		}()

		testServer = httptest.NewServer(handler)
	}()

	return testServer
}

func receiveVaultRequest(t *testing.T, requests <-chan recordedVaultRequest) recordedVaultRequest {
	t.Helper()

	select {
	case request := <-requests:
		return request
	default:
		t.Fatal("fake Vault server did not receive a request")

		return recordedVaultRequest{}
	}
}

func validSecretInformation() *SecretInformation {
	return &SecretInformation{
		MountPath:  "secret",
		SecretPath: "application/config",
	}
}

//////
// Constructor tests.
//////

func TestNewWithEnvironmentConfig(t *testing.T) {
	cleanVaultEnvironment(t)

	server, _, callCount := newVaultTestServer(t, http.StatusInternalServerError, "")
	t.Setenv(api.EnvVaultAddress, server.address)
	t.Setenv(api.EnvVaultToken, "environment-token")

	got, err := New(
		true,
		true,
		&Auth{
			Address:   server.address,
			Namespace: "engineering",
			Token:     "argument-token",
		},
		validSecretInformation(),
	)
	require.NoError(t, err)

	vaultProvider, ok := got.(*Vault)
	require.True(t, ok)
	assert.Equal(t, Name, vaultProvider.GetName())
	assert.True(t, vaultProvider.GetOverride())
	assert.True(t, vaultProvider.GetRawValue())
	assert.Equal(t, "argument-token", vaultProvider.client.Token())
	assert.Equal(t, "engineering", vaultProvider.client.Namespace())
	assert.EqualValues(t, 0, callCount.Load())
}

func TestNewWithConfigValidationFailures(t *testing.T) {
	tests := []struct {
		name            string
		authInformation *Auth
		config          func(t *testing.T) Config
		secret          *SecretInformation
		wantContains    string
	}{
		{
			name:            "missing auth information",
			authInformation: nil,
			secret:          validSecretInformation(),
		},
		{
			name: "missing address",
			authInformation: &Auth{
				Token: "test-token",
			},
			secret: validSecretInformation(),
		},
		{
			name: "missing secret information",
			authInformation: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "test-token",
			},
			secret: nil,
		},
		{
			name: "missing mount path",
			authInformation: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "test-token",
			},
			secret: &SecretInformation{
				SecretPath: "application/config",
			},
		},
		{
			name: "missing secret path",
			authInformation: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "test-token",
			},
			secret: &SecretInformation{
				MountPath: "secret",
			},
		},
		{
			name: "missing token",
			authInformation: &Auth{
				Address: "http://127.0.0.1:8200",
			},
			config: func(t *testing.T) Config {
				t.Helper()

				return newVaultTestConfig(t, "http://127.0.0.1:8200")
			},
			secret:       validSecretInformation(),
			wantContains: "token",
		},
		{
			name: "invalid client config address",
			authInformation: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "test-token",
			},
			config: func(t *testing.T) Config {
				t.Helper()

				return &api.Config{
					Address:    "://invalid",
					HttpClient: &http.Client{},
				}
			},
			secret:       validSecretInformation(),
			wantContains: "initialize Vault client",
		},
		{
			name: "invalid auth address",
			authInformation: &Auth{
				Address: "://invalid",
				Token:   "test-token",
			},
			config: func(t *testing.T) Config {
				t.Helper()

				return newVaultTestConfig(t, "http://127.0.0.1:8200")
			},
			secret:       validSecretInformation(),
			wantContains: "set Vault address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanVaultEnvironment(t)

			var config Config
			if tt.config != nil {
				config = tt.config(t)
			}

			got, err := NewWithConfig(
				false,
				false,
				tt.authInformation,
				tt.secret,
				config,
			)

			assert.Error(t, err)
			assert.Nil(t, got)

			if tt.wantContains != "" {
				assert.ErrorContains(t, err, tt.wantContains)
			}
		})
	}
}

func TestNewWithConfigAppRoleAuthentication(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		roleID       string
		secretID     string
		statusCode   int
		wantContains string
		wantRequest  bool
		wantToken    string
	}{
		{
			name:         "missing role ID",
			roleID:       "",
			secretID:     "secret-id",
			statusCode:   http.StatusOK,
			wantContains: "role_id and secret_id",
		},
		{
			name:         "missing secret ID",
			roleID:       "role-id",
			secretID:     "",
			statusCode:   http.StatusOK,
			wantContains: "role_id and secret_id",
		},
		{
			name:         "login rejected",
			roleID:       "role-id",
			secretID:     "secret-id",
			statusCode:   http.StatusForbidden,
			responseBody: `{"errors":["permission denied"]}`,
			wantContains: "login with approle",
			wantRequest:  true,
		},
		{
			name:         "empty login response",
			roleID:       "role-id",
			secretID:     "secret-id",
			statusCode:   http.StatusNoContent,
			wantContains: "resp",
			wantRequest:  true,
		},
		{
			name:         "login succeeds",
			roleID:       "role-id",
			secretID:     "secret-id",
			statusCode:   http.StatusOK,
			responseBody: `{"auth":{"client_token":"approle-token","lease_duration":3600}}`,
			wantRequest:  true,
			wantToken:    "approle-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanVaultEnvironment(t)

			server, requests, callCount := newVaultTestServer(t, tt.statusCode, tt.responseBody)
			got, err := NewWithConfig(
				false,
				false,
				&Auth{
					Address:   server.address,
					AppRole:   "application",
					Namespace: "engineering",
					RoleID:    tt.roleID,
					SecretID:  tt.secretID,
				},
				validSecretInformation(),
				newVaultTestConfig(t, server.address, server.client),
			)

			if tt.wantContains != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)

				vaultProvider, ok := got.(*Vault)
				require.True(t, ok)
				assert.Equal(t, tt.wantToken, vaultProvider.client.Token())
			}

			if tt.wantRequest {
				request := receiveVaultRequest(t, requests)
				assert.Equal(t, http.MethodPut, request.method)
				assert.Equal(t, "/v1/auth/approle/login", request.path)
				assert.Equal(t, "engineering", request.header.Get(api.NamespaceHeaderName))
				assert.Equal(t, map[string]interface{}{
					"role_id":   tt.roleID,
					"secret_id": tt.secretID,
				}, request.body)
				assert.EqualValues(t, 1, callCount.Load())
			} else {
				assert.EqualValues(t, 0, callCount.Load())
			}
		})
	}
}

//////
// Load tests.
//////

func TestVaultLoad(t *testing.T) {
	const validMetadata = `"metadata":{"created_time":"2024-01-01T00:00:00Z","deletion_time":"","destroyed":false,"version":1}`

	tests := []struct {
		name         string
		opts         []option.LoadKeyFunc
		override     bool
		responseBody string
		statusCode   int
		want         map[string]string
		wantContains string
	}{
		{
			name:         "unwraps secret data and applies key options",
			opts:         []option.LoadKeyFunc{option.WithKeyPrefixer("TEST_"), option.WithKeyCaser(option.Upper)},
			responseBody: `{"data":{"data":{"db_user":"admin","port":5432},` + validMetadata + `}}`,
			statusCode:   http.StatusOK,
			want: map[string]string{
				"TEST_DB_USER": "admin",
				"TEST_PORT":    "5432",
			},
		},
		{
			name:         "preserves an existing environment value",
			responseBody: `{"data":{"data":{"VAULT_EXISTING":"from-vault"},` + validMetadata + `}}`,
			statusCode:   http.StatusOK,
			want: map[string]string{
				"VAULT_EXISTING": "from-environment",
			},
		},
		{
			name:         "loads an empty secret",
			responseBody: `{"data":{"data":{},` + validMetadata + `}}`,
			statusCode:   http.StatusOK,
			want:         map[string]string{},
		},
		{
			name:         "missing secret",
			responseBody: `{"errors":["secret not found"]}`,
			statusCode:   http.StatusNotFound,
			wantContains: "get secret",
		},
		{
			name:         "malformed JSON response",
			responseBody: `{"data":`,
			statusCode:   http.StatusOK,
			wantContains: "get secret",
		},
		{
			name:         "malformed KV response shape",
			responseBody: `{"data":{` + validMetadata + `}}`,
			statusCode:   http.StatusOK,
			wantContains: "get secret",
		},
		{
			name:         "invalid environment key",
			responseBody: `{"data":{"data":{"bad=key":"value"},` + validMetadata + `}}`,
			statusCode:   http.StatusOK,
			wantContains: "export bad=key env var",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanVaultEnvironment(t)
			testenv.Unset(t, "TEST_DB_USER")
			testenv.Unset(t, "TEST_PORT")
			t.Setenv("VAULT_EXISTING", "from-environment")

			server, requests, callCount := newVaultTestServer(t, tt.statusCode, tt.responseBody)
			providerInterface, err := NewWithConfig(
				tt.override,
				false,
				&Auth{
					Address: server.address,
					Token:   "test-token",
				},
				validSecretInformation(),
				newVaultTestConfig(t, server.address, server.client),
			)
			require.NoError(t, err)

			got, err := providerInterface.Load(context.Background(), tt.opts...)
			if tt.wantContains != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)

				for key, value := range tt.want {
					assert.Equal(t, value, os.Getenv(key))
				}
			}

			request := receiveVaultRequest(t, requests)
			assert.Equal(t, http.MethodGet, request.method)
			assert.Equal(t, "/v1/secret/data/application/config", request.path)
			assert.Equal(t, "test-token", request.header.Get("X-Vault-Token"))
			assert.EqualValues(t, 1, callCount.Load())
		})
	}
}

//////
// Write tests.
//////

func TestVaultWrite(t *testing.T) {
	tests := []struct {
		name         string
		opts         []option.WriteFunc
		responseBody string
		statusCode   int
		values       map[string]interface{}
		wantContains string
		wantRequest  bool
	}{
		{
			name:         "writes secret data",
			opts:         []option.WriteFunc{option.WithVariable(true)},
			responseBody: `{"data":{"created_time":"2024-01-01T00:00:00Z","deletion_time":"","destroyed":false,"version":1}}`,
			statusCode:   http.StatusOK,
			values: map[string]interface{}{
				"password": "secret",
				"retries":  float64(3),
			},
			wantRequest: true,
		},
		{
			name:         "rejects nil values",
			statusCode:   http.StatusOK,
			values:       nil,
			wantContains: "values",
		},
		{
			name:         "returns write option error",
			opts:         []option.WriteFunc{option.WithTarget("")},
			statusCode:   http.StatusOK,
			values:       map[string]interface{}{},
			wantContains: "target",
		},
		{
			name:         "Vault rejects write",
			responseBody: `{"errors":["permission denied"]}`,
			statusCode:   http.StatusForbidden,
			values: map[string]interface{}{
				"password": "secret",
			},
			wantContains: "write secret",
			wantRequest:  true,
		},
		{
			name:       "Vault returns empty write response",
			statusCode: http.StatusNoContent,
			values: map[string]interface{}{
				"password": "secret",
			},
			wantContains: "write secret",
			wantRequest:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanVaultEnvironment(t)

			server, requests, callCount := newVaultTestServer(t, tt.statusCode, tt.responseBody)
			providerInterface, err := NewWithConfig(
				false,
				false,
				&Auth{
					Address: server.address,
					Token:   "test-token",
				},
				validSecretInformation(),
				newVaultTestConfig(t, server.address, server.client),
			)
			require.NoError(t, err)

			err = providerInterface.Write(context.Background(), tt.values, tt.opts...)
			if tt.wantContains != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantRequest {
				request := receiveVaultRequest(t, requests)
				assert.Equal(t, http.MethodPut, request.method)
				assert.Equal(t, "/v1/secret/data/application/config", request.path)
				assert.Equal(t, "test-token", request.header.Get("X-Vault-Token"))
				assert.Equal(t, map[string]interface{}{
					"data": tt.values,
				}, request.body)
				assert.EqualValues(t, 1, callCount.Load())
			} else {
				assert.EqualValues(t, 0, callCount.Load())
			}
		})
	}
}
