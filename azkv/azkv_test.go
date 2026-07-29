package azkv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/option"
)

//////
// Test helpers.
//////

type fakeCredential struct{}

func (fakeCredential) GetToken(
	context.Context,
	policy.TokenRequestOptions,
) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     "test-token",
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

type fakeResponse struct {
	status int
	body   string
}

type recordedRequest struct {
	method string
	path   string
	body   map[string]interface{}
}

type fakeKeyVault struct {
	t         *testing.T
	server    *httptest.Server
	client    *http.Client
	mu        sync.Mutex
	responses map[string][]fakeResponse
	requests  []recordedRequest
}

func newFakeKeyVault(t *testing.T) *fakeKeyVault {
	t.Helper()

	fake := &fakeKeyVault{
		t:         t,
		responses: make(map[string][]fakeResponse),
	}

	handler := http.HandlerFunc(fake.serveHTTP)
	fake.server = tryNewTLSTestServer(handler)
	if fake.server != nil {
		fake.client = fake.server.Client()
	} else {
		listener := newMemoryListener()
		fake.server = &httptest.Server{
			Listener: listener,
			Config:   &http.Server{Handler: handler},
		}
		fake.server.StartTLS()

		baseTransport, ok := fake.server.Client().Transport.(*http.Transport)
		if !ok {
			t.Fatal("unexpected transport type")
		}

		transport := baseTransport.Clone()
		transport.DialContext = listener.dialContext
		fake.client = &http.Client{Transport: transport}
	}

	t.Cleanup(func() {
		fake.client.CloseIdleConnections()
		fake.server.Close()
	})

	return fake
}

func tryNewTLSTestServer(handler http.Handler) *httptest.Server {
	var testServer *httptest.Server

	func() {
		defer func() {
			if recover() != nil {
				testServer = nil
			}
		}()

		testServer = httptest.NewTLSServer(handler)
	}()

	return testServer
}

type memoryListener struct {
	connections chan net.Conn
	closed      chan struct{}
	closeOnce   sync.Once
}

func newMemoryListener() *memoryListener {
	return &memoryListener{
		connections: make(chan net.Conn),
		closed:      make(chan struct{}),
	}
}

func (l *memoryListener) Accept() (net.Conn, error) {
	select {
	case connection := <-l.connections:
		return connection, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *memoryListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.closed)
	})

	return nil
}

func (l *memoryListener) Addr() net.Addr {
	return memoryAddress{}
}

func (l *memoryListener) dialContext(
	ctx context.Context,
	_, _ string,
) (net.Conn, error) {
	client, server := net.Pipe()

	select {
	case l.connections <- server:
		return client, nil
	case <-ctx.Done():
		_ = client.Close()
		_ = server.Close()

		return nil, ctx.Err()
	case <-l.closed:
		_ = client.Close()
		_ = server.Close()

		return nil, net.ErrClosed
	}
}

type memoryAddress struct{}

func (memoryAddress) Network() string {
	return "memory"
}

func (memoryAddress) String() string {
	return "example.com"
}

func (f *fakeKeyVault) enqueue(method, path string, status int, body string) {
	f.t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	key := method + " " + normalizePath(path)
	f.responses[key] = append(f.responses[key], fakeResponse{
		status: status,
		body:   body,
	})
}

func (f *fakeKeyVault) serveHTTP(w http.ResponseWriter, request *http.Request) {
	if request.Header.Get("Authorization") == "" {
		w.Header().Set(
			"WWW-Authenticate",
			`Bearer authorization="https://login.microsoftonline.com/test-tenant", resource="https://vault.azure.net"`,
		)
		w.WriteHeader(http.StatusUnauthorized)

		return
	}

	body := map[string]interface{}{}
	if request.Body != nil {
		data, err := io.ReadAll(request.Body)
		if err != nil {
			f.t.Errorf("read request body: %v", err)
		}

		if len(data) > 0 {
			if err := json.Unmarshal(data, &body); err != nil {
				f.t.Errorf("decode request body %q: %v", data, err)
			}
		}
	}

	key := request.Method + " " + normalizePath(request.URL.Path)

	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{
		method: request.Method,
		path:   normalizePath(request.URL.Path),
		body:   body,
	})

	responses := f.responses[key]
	if len(responses) == 0 {
		f.mu.Unlock()
		f.t.Errorf("unexpected Azure Key Vault request %s", key)
		http.Error(w, "unexpected request", http.StatusInternalServerError)

		return
	}

	response := responses[0]
	f.responses[key] = responses[1:]
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.status)

	if _, err := io.WriteString(w, response.body); err != nil {
		f.t.Errorf("write response: %v", err)
	}
}

func (f *fakeKeyVault) recordedRequests() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]recordedRequest(nil), f.requests...)
}

func normalizePath(path string) string {
	if path == "/" {
		return path
	}

	return strings.TrimSuffix(path, "/")
}

func newTestAZKV(
	t *testing.T,
	fake *fakeKeyVault,
	override, rawValue bool,
	secretNames ...string,
) *AZKV {
	t.Helper()

	clientOptions := &azsecrets.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries: -1,
			},
			Transport: fake.client,
		},
		DisableChallengeResourceVerification: true,
	}

	got, err := NewWithConfig(
		override,
		rawValue,
		&Config{
			VaultURL:    fake.server.URL,
			SecretNames: secretNames,
		},
		fakeCredential{},
		clientOptions,
	)
	require.NoError(t, err)

	provider, ok := got.(*AZKV)
	require.True(t, ok)

	return provider
}

func secretResponse(vaultURL, name string, value *string) string {
	response := map[string]interface{}{
		"id": vaultURL + "/secrets/" + name + "/version",
	}
	if value != nil {
		response["value"] = *value
	}

	data, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}

	return string(data)
}

func stringPointer(value string) *string {
	return &value
}

//////
// Constructor tests.
//////

func TestNew(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		override        bool
		rawValue        bool
		wantErrContains string
	}{
		{
			name: "happy path creates provider with default Azure credential",
			config: &Config{
				VaultURL:    "https://vault.example.test",
				SecretNames: []string{"app-secret"},
			},
			override: true,
			rawValue: true,
		},
		{
			name:            "bad path rejects nil config",
			wantErrContains: "config",
		},
		{
			name:            "bad path rejects missing vault URL",
			config:          &Config{},
			wantErrContains: "VaultURL",
		},
		{
			name: "edge case rejects blank configured secret name",
			config: &Config{
				VaultURL:    "https://vault.example.test",
				SecretNames: []string{""},
			},
			wantErrContains: "SecretNames",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.override, tt.rawValue, tt.config)

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, Name, got.GetName())
			assert.Equal(t, tt.override, got.GetOverride())
			assert.Equal(t, tt.rawValue, got.GetRawValue())
		})
	}
}

func TestNewWithConfig(t *testing.T) {
	validConfig := &Config{VaultURL: "https://vault.example.test"}

	tests := []struct {
		name            string
		config          *Config
		credential      azcore.TokenCredential
		override        bool
		rawValue        bool
		wantErrContains string
	}{
		{
			name:       "happy path preserves provider options",
			config:     validConfig,
			credential: fakeCredential{},
			override:   true,
			rawValue:   true,
		},
		{
			name:            "bad path rejects nil config",
			credential:      fakeCredential{},
			wantErrContains: "config",
		},
		{
			name:            "bad path rejects nil credential",
			config:          validConfig,
			wantErrContains: "credential",
		},
		{
			name: "edge case rejects malformed vault URL",
			config: &Config{
				VaultURL: "://not-a-url",
			},
			credential:      fakeCredential{},
			wantErrContains: "VaultURL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewWithConfig(
				tt.override,
				tt.rawValue,
				tt.config,
				tt.credential,
				nil,
			)

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.override, got.GetOverride())
			assert.Equal(t, tt.rawValue, got.GetRawValue())
		})
	}
}

//////
// Load tests.
//////

func TestAZKVLoadConfiguredSecrets(t *testing.T) {
	tests := []struct {
		name            string
		secretName      string
		responseStatus  int
		responseValue   *string
		responseBody    string
		override        bool
		rawValue        bool
		opts            []option.LoadKeyFunc
		existingKey     string
		existingValue   string
		wantValues      map[string]string
		wantErrContains []string
	}{
		{
			name:           "happy path flattens JSON object and applies key functions",
			secretName:     "app-json",
			responseStatus: http.StatusOK,
			responseValue:  stringPointer(`{"db_host":"db.example.test","port":5432}`),
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AZKV_TEST_"),
				option.WithKeyCaser("upper"),
			},
			wantValues: map[string]string{
				"AZKV_TEST_DB_HOST": "db.example.test",
				"AZKV_TEST_PORT":    "5432",
			},
		},
		{
			name:           "happy path maps dashes in a plain secret name to underscores",
			secretName:     "plain-secret",
			responseStatus: http.StatusOK,
			responseValue:  stringPointer("plain-value"),
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AZKV_TEST_"),
				option.WithKeyCaser("upper"),
			},
			wantValues: map[string]string{
				"AZKV_TEST_PLAIN_SECRET": "plain-value",
			},
		},
		{
			name:           "happy path flattens a double-encoded JSON object",
			secretName:     "wrapped-json",
			responseStatus: http.StatusOK,
			responseValue:  stringPointer(`"{\"AZKV_TEST_WRAPPED\":\"wrapped-value\"}"`),
			wantValues: map[string]string{
				"AZKV_TEST_WRAPPED": "wrapped-value",
			},
		},
		{
			name:           "happy path preserves an existing environment value without override",
			secretName:     "existing",
			responseStatus: http.StatusOK,
			responseValue:  stringPointer("from-vault"),
			existingKey:    "AZKV_TEST_EXISTING",
			existingValue:  "from-environment",
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AZKV_TEST_"),
				option.WithKeyCaser("upper"),
			},
			wantValues: map[string]string{
				"AZKV_TEST_EXISTING": "from-environment",
			},
		},
		{
			name:           "happy path overrides an existing environment value",
			secretName:     "existing",
			responseStatus: http.StatusOK,
			responseValue:  stringPointer("from-vault"),
			override:       true,
			existingKey:    "AZKV_TEST_EXISTING",
			existingValue:  "from-environment",
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AZKV_TEST_"),
				option.WithKeyCaser("upper"),
			},
			wantValues: map[string]string{
				"AZKV_TEST_EXISTING": "from-vault",
			},
		},
		{
			name:           "edge case raw value preserves Go quoted representation",
			secretName:     "raw-secret",
			responseStatus: http.StatusOK,
			responseValue:  stringPointer("line\nvalue"),
			rawValue:       true,
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AZKV_TEST_"),
				option.WithKeyCaser("upper"),
			},
			wantValues: map[string]string{
				"AZKV_TEST_RAW_SECRET": `"line\nvalue"`,
			},
		},
		{
			name:           "edge case raw value skips JSON object flattening",
			secretName:     "raw-json-secret",
			responseStatus: http.StatusOK,
			responseValue:  stringPointer(`{"db_host":"db.example.test","port":5432}`),
			rawValue:       true,
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AZKV_TEST_"),
				option.WithKeyCaser("upper"),
			},
			wantValues: map[string]string{
				"AZKV_TEST_RAW_JSON_SECRET": `"{\"db_host\":\"db.example.test\",\"port\":5432}"`,
			},
		},
		{
			name:           "edge case exports an empty plain value",
			secretName:     "empty-secret",
			responseStatus: http.StatusOK,
			responseValue:  stringPointer(""),
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AZKV_TEST_"),
				option.WithKeyCaser("upper"),
			},
			wantValues: map[string]string{
				"AZKV_TEST_EMPTY_SECRET": "",
			},
		},
		{
			name:            "bad path reports missing secret",
			secretName:      "missing-secret",
			responseStatus:  http.StatusNotFound,
			responseValue:   stringPointer(`{"error":{"code":"SecretNotFound"}}`),
			wantErrContains: []string{"get secret 'missing-secret'"},
		},
		{
			name:            "bad path reports Azure API error",
			secretName:      "api-error",
			responseStatus:  http.StatusInternalServerError,
			responseValue:   stringPointer(`{"error":{"code":"InternalServerError"}}`),
			wantErrContains: []string{"get secret 'api-error'"},
		},
		{
			name:           "bad path wraps malformed Azure response decode error",
			secretName:     "malformed-response",
			responseStatus: http.StatusOK,
			responseBody:   `{`,
			wantErrContains: []string{
				"get secret 'malformed-response'",
				"unexpected end of JSON input",
			},
		},
		{
			name:            "bad path rejects malformed payload missing value",
			secretName:      "malformed",
			responseStatus:  http.StatusOK,
			wantErrContains: []string{"payload is missing value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeKeyVault(t)

			body := secretResponse(fake.server.URL, tt.secretName, tt.responseValue)
			if tt.responseStatus >= http.StatusBadRequest && tt.responseValue != nil {
				body = *tt.responseValue
			}
			if tt.responseBody != "" {
				body = tt.responseBody
			}
			fake.enqueue(
				http.MethodGet,
				"/secrets/"+tt.secretName,
				tt.responseStatus,
				body,
			)

			for key := range tt.wantValues {
				t.Setenv(key, "")
			}
			if tt.existingKey != "" {
				t.Setenv(tt.existingKey, tt.existingValue)
			}

			provider := newTestAZKV(
				t,
				fake,
				tt.override,
				tt.rawValue,
				tt.secretName,
			)

			got, err := provider.Load(context.Background(), tt.opts...)
			if len(tt.wantErrContains) > 0 {
				require.Error(t, err)
				for _, wantErrContains := range tt.wantErrContains {
					assert.Contains(t, err.Error(), wantErrContains)
				}
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValues, got)
			for key, value := range tt.wantValues {
				assert.Equal(t, value, os.Getenv(key))
			}
		})
	}
}

func TestAZKVLoadListsAllSecrets(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(*testing.T, *fakeKeyVault)
		wantValues      map[string]string
		wantErrContains string
	}{
		{
			name: "happy path lists and retrieves every secret",
			setup: func(t *testing.T, fake *fakeKeyVault) {
				t.Helper()

				fake.enqueue(
					http.MethodGet,
					"/secrets",
					http.StatusOK,
					fmt.Sprintf(
						`{"value":[{"id":%q},{"id":%q}]}`,
						fake.server.URL+"/secrets/json-secret",
						fake.server.URL+"/secrets/plain-secret",
					),
				)
				fake.enqueue(
					http.MethodGet,
					"/secrets/json-secret",
					http.StatusOK,
					secretResponse(
						fake.server.URL,
						"json-secret",
						stringPointer(`{"AZKV_TEST_LISTED_JSON":"json-value"}`),
					),
				)
				fake.enqueue(
					http.MethodGet,
					"/secrets/plain-secret",
					http.StatusOK,
					secretResponse(
						fake.server.URL,
						"plain-secret",
						stringPointer("plain-value"),
					),
				)
			},
			wantValues: map[string]string{
				"AZKV_TEST_LISTED_JSON": "json-value",
				"plain_secret":          "plain-value",
			},
		},
		{
			name: "edge case follows list pagination",
			setup: func(t *testing.T, fake *fakeKeyVault) {
				t.Helper()

				fake.enqueue(
					http.MethodGet,
					"/secrets",
					http.StatusOK,
					fmt.Sprintf(
						`{"value":[],"nextLink":%q}`,
						fake.server.URL+"/next-secrets",
					),
				)
				fake.enqueue(
					http.MethodGet,
					"/next-secrets",
					http.StatusOK,
					fmt.Sprintf(
						`{"value":[{"id":%q}]}`,
						fake.server.URL+"/secrets/paged-secret",
					),
				)
				fake.enqueue(
					http.MethodGet,
					"/secrets/paged-secret",
					http.StatusOK,
					secretResponse(
						fake.server.URL,
						"paged-secret",
						stringPointer("paged-value"),
					),
				)
			},
			wantValues: map[string]string{
				"paged_secret": "paged-value",
			},
		},
		{
			name: "edge case empty vault returns an empty map",
			setup: func(t *testing.T, fake *fakeKeyVault) {
				t.Helper()

				fake.enqueue(
					http.MethodGet,
					"/secrets",
					http.StatusOK,
					`{"value":[]}`,
				)
			},
			wantValues: map[string]string{},
		},
		{
			name: "bad path reports list API error",
			setup: func(t *testing.T, fake *fakeKeyVault) {
				t.Helper()

				fake.enqueue(
					http.MethodGet,
					"/secrets",
					http.StatusInternalServerError,
					`{"error":{"code":"InternalServerError"}}`,
				)
			},
			wantErrContains: "list secret properties",
		},
		{
			name: "bad path rejects malformed listed secret",
			setup: func(t *testing.T, fake *fakeKeyVault) {
				t.Helper()

				fake.enqueue(
					http.MethodGet,
					"/secrets",
					http.StatusOK,
					`{"value":[{}]}`,
				)
			},
			wantErrContains: "missing ID",
		},
		{
			name: "bad path rejects invalid listed secret ID",
			setup: func(t *testing.T, fake *fakeKeyVault) {
				t.Helper()

				fake.enqueue(
					http.MethodGet,
					"/secrets",
					http.StatusOK,
					`{"value":[{"id":"not-an-azure-secret-id"}]}`,
				)
			},
			wantErrContains: "invalid ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeKeyVault(t)
			tt.setup(t, fake)

			for key := range tt.wantValues {
				t.Setenv(key, "")
			}

			provider := newTestAZKV(t, fake, false, false)
			got, err := provider.Load(context.Background())

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValues, got)
		})
	}
}

//////
// Write tests.
//////

func TestAZKVWrite(t *testing.T) {
	failingOption := func(*option.Write) error {
		return fmt.Errorf("write option failed")
	}

	tests := []struct {
		name            string
		values          map[string]interface{}
		opts            []option.WriteFunc
		setup           func(*testing.T, *fakeKeyVault)
		wantRequests    map[string]interface{}
		wantErrContains string
	}{
		{
			name: "happy path writes every value and maps underscores to dashes",
			values: map[string]interface{}{
				"APP_KEY": "secret-value",
				"PORT":    443,
			},
			setup: func(t *testing.T, fake *fakeKeyVault) {
				t.Helper()

				fake.enqueue(
					http.MethodPut,
					"/secrets/APP-KEY",
					http.StatusOK,
					secretResponse(
						fake.server.URL,
						"APP-KEY",
						stringPointer("secret-value"),
					),
				)
				fake.enqueue(
					http.MethodPut,
					"/secrets/PORT",
					http.StatusOK,
					secretResponse(
						fake.server.URL,
						"PORT",
						stringPointer("443"),
					),
				)
			},
			wantRequests: map[string]interface{}{
				"APP-KEY": "secret-value",
				"PORT":    "443",
			},
		},
		{
			name:         "edge case empty values is a successful no-op",
			values:       map[string]interface{}{},
			setup:        func(*testing.T, *fakeKeyVault) {},
			wantRequests: map[string]interface{}{},
		},
		{
			name:            "bad path rejects nil values",
			wantErrContains: "values",
			setup:           func(*testing.T, *fakeKeyVault) {},
		},
		{
			name: "bad path propagates write option error",
			values: map[string]interface{}{
				"KEY": "value",
			},
			opts:            []option.WriteFunc{failingOption},
			wantErrContains: "write option failed",
			setup:           func(*testing.T, *fakeKeyVault) {},
		},
		{
			name: "bad path rejects empty secret name",
			values: map[string]interface{}{
				"": "value",
			},
			wantErrContains: "secret name can't be empty",
			setup:           func(*testing.T, *fakeKeyVault) {},
		},
		{
			name: "bad path reports Azure set error",
			values: map[string]interface{}{
				"FAIL_KEY": "value",
			},
			setup: func(t *testing.T, fake *fakeKeyVault) {
				t.Helper()

				fake.enqueue(
					http.MethodPut,
					"/secrets/FAIL-KEY",
					http.StatusInternalServerError,
					`{"error":{"code":"InternalServerError"}}`,
				)
			},
			wantErrContains: "set secret 'FAIL-KEY'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeKeyVault(t)
			tt.setup(t, fake)

			provider := newTestAZKV(t, fake, false, false)
			err := provider.Write(context.Background(), tt.values, tt.opts...)

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)

				return
			}

			require.NoError(t, err)

			requestValues := make(map[string]interface{})
			for _, request := range fake.recordedRequests() {
				if request.method != http.MethodPut {
					continue
				}

				name := strings.TrimPrefix(request.path, "/secrets/")
				requestValues[name] = request.body["value"]
			}
			assert.Equal(t, tt.wantRequests, requestValues)
		})
	}
}

//////
// Parsing tests.
//////

func TestParseSecretData(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  map[string]interface{}
		ok    bool
	}{
		{
			name:  "happy path JSON object",
			value: `{"KEY":"value"}`,
			want:  map[string]interface{}{"KEY": "value"},
			ok:    true,
		},
		{
			name:  "happy path double-encoded JSON object",
			value: `"{\"KEY\":\"value\"}"`,
			want:  map[string]interface{}{"KEY": "value"},
			ok:    true,
		},
		{
			name:  "edge case empty JSON object",
			value: `{}`,
			want:  map[string]interface{}{},
			ok:    true,
		},
		{
			name:  "edge case JSON null is a plain value",
			value: `null`,
		},
		{
			name:  "edge case JSON array is a plain value",
			value: `["value"]`,
		},
		{
			name:  "bad path malformed JSON object is a plain value",
			value: `{"KEY":`,
		},
		{
			name:  "happy path genuine plain text",
			value: "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSecretData(tt.value)

			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
