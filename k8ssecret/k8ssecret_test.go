package k8ssecret

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/internal/testenv"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
)

//////
// Constructor.
//////

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		host          string
		port          string
		wantErr       string
		wantAPIServer string
		wantNamespace string
	}{
		{
			name:          "happy path mounted mode",
			config:        &Config{Path: "/mounted/secrets"},
			wantNamespace: "default",
		},
		{
			name: "happy path API mode",
			config: &Config{
				APIServer:  "https://kubernetes.example.test",
				Namespace:  "application",
				SecretName: "config",
				Token:      "token",
			},
			wantAPIServer: "https://kubernetes.example.test",
			wantNamespace: "application",
		},
		{
			name: "edge derives in-cluster API server",
			config: &Config{
				SecretName: "config",
				Token:      "token",
			},
			host:          "10.0.0.1",
			port:          "6443",
			wantAPIServer: "https://10.0.0.1:6443",
			wantNamespace: "default",
		},
		{
			name: "edge defaults in-cluster port",
			config: &Config{
				SecretName: "config",
				Token:      "token",
			},
			host:          "kubernetes.default.svc",
			wantAPIServer: "https://kubernetes.default.svc:443",
			wantNamespace: "default",
		},
		{
			name:    "bad path nil config",
			wantErr: "config",
		},
		{
			name:    "bad path missing mounted path and API server",
			config:  &Config{},
			wantErr: "path or API server",
		},
		{
			name: "bad path API mode missing secret name",
			config: &Config{
				APIServer: "https://kubernetes.example.test",
				Token:     "token",
			},
			wantErr: "secret name",
		},
		{
			name: "bad path API mode missing token",
			config: &Config{
				APIServer:  "https://kubernetes.example.test",
				SecretName: "config",
			},
			wantErr: "token or token file",
		},
		{
			name: "bad path invalid API server",
			config: &Config{
				APIServer:  "not-a-url",
				SecretName: "config",
				Token:      "token",
			},
			wantErr: "APIServer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tt.host)
			t.Setenv("KUBERNETES_SERVICE_PORT", tt.port)

			created, err := New(false, false, tt.config)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, created)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, created)
			assert.Equal(t, Name, created.GetName())

			typed, ok := created.(*K8sSecret)
			require.True(t, ok)
			assert.Equal(t, tt.wantAPIServer, typed.Configuration.APIServer)
			assert.Equal(t, tt.wantNamespace, typed.Configuration.Namespace)
		})
	}
}

func TestNewCACertificateErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T) string
		wantErr string
	}{
		{
			name: "bad path missing CA certificate",
			setup: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "missing-ca.crt")
			},
			wantErr: "read Kubernetes CA certificate",
		},
		{
			name: "bad path invalid CA certificate",
			setup: func(t *testing.T) string {
				t.Helper()

				path := filepath.Join(t.TempDir(), "ca.crt")
				require.NoError(t, os.WriteFile(path, []byte("not a certificate"), 0o600))

				return path
			},
			wantErr: "Kubernetes CA certificate is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := New(false, false, &Config{
				APIServer:  "https://kubernetes.example.test",
				SecretName: "config",
				Token:      "token",
				CACertFile: tt.setup(t),
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Nil(t, created)
		})
	}
}

//////
// Mounted-secret load mode.
//////

func TestK8sSecretLoadMounted(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T) string
		opts    []option.LoadKeyFunc
		want    map[string]string
		wantErr string
	}{
		{
			name: "happy path reads regular files trims one line ending and applies key options",
			setup: func(t *testing.T) string {
				t.Helper()

				directory := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(directory, "api_key"),
					[]byte("secret\r\n"),
					0o600,
				))
				require.NoError(t, os.WriteFile(
					filepath.Join(directory, "MULTILINE"),
					[]byte("first\nsecond\n\n"),
					0o600,
				))

				return directory
			},
			opts: []option.LoadKeyFunc{
				option.WithKeyCaser("upper"),
				option.WithKeyPrefixer("K8S_MOUNT_"),
			},
			want: map[string]string{
				"K8S_MOUNT_API_KEY":   "secret",
				"K8S_MOUNT_MULTILINE": "first\nsecond\n",
			},
		},
		{
			name: "edge skips dotfiles subdirectories and follows visible key symlinks",
			setup: func(t *testing.T) string {
				t.Helper()

				directory := t.TempDir()
				dataDirectory := filepath.Join(directory, "..data")
				require.NoError(t, os.Mkdir(dataDirectory, 0o700))
				require.NoError(t, os.WriteFile(
					filepath.Join(dataDirectory, "LINKED_KEY"),
					[]byte("linked-value\n"),
					0o600,
				))
				require.NoError(t, os.Symlink(
					filepath.Join("..data", "LINKED_KEY"),
					filepath.Join(directory, "LINKED_KEY"),
				))
				require.NoError(t, os.WriteFile(
					filepath.Join(directory, ".ignored"),
					[]byte("ignored"),
					0o600,
				))
				require.NoError(t, os.Mkdir(filepath.Join(directory, "subdirectory"), 0o700))

				return directory
			},
			want: map[string]string{
				"LINKED_KEY": "linked-value",
			},
		},
		{
			name: "edge empty directory returns empty map",
			setup: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			want: map[string]string{},
		},
		{
			name: "bad path missing directory",
			setup: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "missing")
			},
			wantErr: "read mounted Kubernetes secret directory",
		},
		{
			name: "bad path dangling visible symlink",
			setup: func(t *testing.T) string {
				t.Helper()

				directory := t.TempDir()
				require.NoError(t, os.Symlink(
					"missing-target",
					filepath.Join(directory, "BROKEN_KEY"),
				))

				return directory
			},
			wantErr: "stat mounted Kubernetes secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key := range tt.want {
				testenv.Unset(t, key)
			}

			created, err := New(false, false, &Config{Path: tt.setup(t)})
			require.NoError(t, err)

			got, err := created.Load(context.Background(), tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			for key, value := range tt.want {
				assert.Equal(t, value, os.Getenv(key))
			}
		})
	}
}

func TestK8sSecretMountedWriteNotSupported(t *testing.T) {
	created, err := New(false, false, &Config{Path: t.TempDir()})
	require.NoError(t, err)

	err = created.Write(context.Background(), map[string]interface{}{"KEY": "value"})

	require.Error(t, err)
	assert.ErrorIs(t, err, provider.ErrNotSupported)
}

//////
// API load mode.
//////

func TestK8sSecretLoadAPI(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		token        string
		tokenFile    string
		removeToken  bool
		opts         []option.LoadKeyFunc
		want         map[string]string
		wantErr      string
		wantRequests int
	}{
		{
			name:         "happy path decodes base64 values and applies key options",
			statusCode:   http.StatusOK,
			responseBody: `{"data":{"api_key":"c2VjcmV0","port":"ODA4MA=="}}`,
			token:        "api-token",
			opts: []option.LoadKeyFunc{
				option.WithKeyCaser("upper"),
				option.WithKeyPrefixer("K8S_API_"),
			},
			want: map[string]string{
				"K8S_API_API_KEY": "secret",
				"K8S_API_PORT":    "8080",
			},
			wantRequests: 1,
		},
		{
			name:         "happy path reads token from file",
			statusCode:   http.StatusOK,
			responseBody: `{"data":{"K8S_TOKEN_FILE_KEY":"dmFsdWU="}}`,
			tokenFile:    "file-token\n",
			want: map[string]string{
				"K8S_TOKEN_FILE_KEY": "value",
			},
			wantRequests: 1,
		},
		{
			name:         "edge missing data returns empty map",
			statusCode:   http.StatusOK,
			responseBody: `{}`,
			token:        "api-token",
			want:         map[string]string{},
			wantRequests: 1,
		},
		{
			name:         "bad path unauthorized",
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"message":"unauthorized"}`,
			token:        "api-token",
			wantErr:      "401 Unauthorized",
			wantRequests: 1,
		},
		{
			name:         "bad path missing secret",
			statusCode:   http.StatusNotFound,
			responseBody: "",
			token:        "api-token",
			wantErr:      "404 Not Found",
			wantRequests: 1,
		},
		{
			name:         "bad path server error",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"message":"failed"}`,
			token:        "api-token",
			wantErr:      "500 Internal Server Error",
			wantRequests: 1,
		},
		{
			name:         "bad path malformed JSON",
			statusCode:   http.StatusOK,
			responseBody: `{"data":`,
			token:        "api-token",
			wantErr:      "decode Kubernetes secret",
			wantRequests: 1,
		},
		{
			name:         "bad path invalid base64 value",
			statusCode:   http.StatusOK,
			responseBody: `{"data":{"BROKEN":"not-base64!"}}`,
			token:        "api-token",
			wantErr:      "decode Kubernetes secret key 'BROKEN'",
			wantRequests: 1,
		},
		{
			name:         "bad path secret key cannot be exported",
			statusCode:   http.StatusOK,
			responseBody: `{"data":{"INVALID=KEY":"dmFsdWU="}}`,
			token:        "api-token",
			wantErr:      "export INVALID=KEY env var",
			wantRequests: 1,
		},
		{
			name:         "bad path token file disappears before request",
			statusCode:   http.StatusOK,
			responseBody: `{"data":{}}`,
			tokenFile:    "temporary-token",
			removeToken:  true,
			wantErr:      "read Kubernetes token file",
			wantRequests: 0,
		},
		{
			name:         "bad path token file is empty",
			statusCode:   http.StatusOK,
			responseBody: `{"data":{}}`,
			tokenFile:    "\n",
			wantErr:      "Kubernetes token file is empty",
			wantRequests: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int
			expectedToken := tt.token

			server, listener := newK8sSecretTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/namespaces/testing/secrets/application", r.URL.Path)
				assert.Equal(t, "Bearer "+expectedToken, r.Header.Get("Authorization"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}), false)

			config := &Config{
				APIServer:  server.URL,
				Namespace:  "testing",
				SecretName: "application",
				Token:      tt.token,
			}

			if tt.tokenFile != "" {
				tokenPath := filepath.Join(t.TempDir(), "token")
				require.NoError(t, os.WriteFile(tokenPath, []byte(tt.tokenFile), 0o600))
				config.TokenFile = tokenPath
				expectedToken = strings.TrimSpace(tt.tokenFile)

				if tt.removeToken {
					require.NoError(t, os.Remove(tokenPath))
				}
			}

			for key := range tt.want {
				testenv.Unset(t, key)
			}

			created, err := New(false, false, config)
			require.NoError(t, err)
			useK8sSecretTestServer(t, created, listener)

			got, err := created.Load(context.Background(), tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			assert.Equal(t, tt.wantRequests, requests)
		})
	}
}

func TestK8sSecretLoadAPITLS(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*testing.T, *httptest.Server, *Config)
	}{
		{
			name: "happy path custom CA certificate",
			configure: func(t *testing.T, server *httptest.Server, config *Config) {
				t.Helper()

				certificate := pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: server.Certificate().Raw,
				})
				path := filepath.Join(t.TempDir(), "ca.crt")
				require.NoError(t, os.WriteFile(path, certificate, 0o600))
				config.CACertFile = path
			},
		},
		{
			name: "edge insecure TLS verification",
			configure: func(_ *testing.T, _ *httptest.Server, config *Config) {
				config.InsecureSkipTLSVerify = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testenv.Unset(t, "K8S_TLS_KEY")

			server, listener := newK8sSecretTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer api-token", r.Header.Get("Authorization"))
				_, _ = w.Write([]byte(`{"data":{"K8S_TLS_KEY":"dmFsdWU="}}`))
			}), true)

			config := &Config{
				APIServer:  server.URL,
				SecretName: "application",
				Token:      "api-token",
			}
			tt.configure(t, server, config)

			created, err := New(false, false, config)
			require.NoError(t, err)
			useK8sSecretTestServer(t, created, listener)

			got, err := created.Load(context.Background())

			require.NoError(t, err)
			assert.Equal(t, map[string]string{"K8S_TLS_KEY": "value"}, got)
		})
	}
}

//////
// API write mode.
//////

func TestK8sSecretWriteAPI(t *testing.T) {
	type capturedRequest struct {
		method      string
		path        string
		contentType string
		body        string
	}

	tests := []struct {
		name         string
		values       map[string]interface{}
		opts         []option.WriteFunc
		patchStatus  int
		patchBody    string
		createStatus int
		createBody   string
		wantErr      string
		wantRequests []capturedRequest
	}{
		{
			name:        "happy path patches base64 encoded values",
			values:      map[string]interface{}{"KEY": "value", "PORT": 8080},
			patchStatus: http.StatusOK,
			wantRequests: []capturedRequest{
				{
					method:      http.MethodPatch,
					path:        "/api/v1/namespaces/testing/secrets/application",
					contentType: "application/strategic-merge-patch+json",
					body:        `{"data":{"KEY":"dmFsdWU=","PORT":"ODA4MA=="}}`,
				},
			},
		},
		{
			name:        "edge patches empty map",
			values:      map[string]interface{}{},
			patchStatus: http.StatusNoContent,
			wantRequests: []capturedRequest{
				{
					method:      http.MethodPatch,
					path:        "/api/v1/namespaces/testing/secrets/application",
					contentType: "application/strategic-merge-patch+json",
					body:        `{"data":{}}`,
				},
			},
		},
		{
			name:         "happy path creates secret after patch not found",
			values:       map[string]interface{}{"KEY": "value"},
			patchStatus:  http.StatusNotFound,
			createStatus: http.StatusCreated,
			wantRequests: []capturedRequest{
				{
					method:      http.MethodPatch,
					path:        "/api/v1/namespaces/testing/secrets/application",
					contentType: "application/strategic-merge-patch+json",
					body:        `{"data":{"KEY":"dmFsdWU="}}`,
				},
				{
					method:      http.MethodPost,
					path:        "/api/v1/namespaces/testing/secrets",
					contentType: "application/json",
					body: `{
						"apiVersion":"v1",
						"kind":"Secret",
						"metadata":{"name":"application"},
						"data":{"KEY":"dmFsdWU="}
					}`,
				},
			},
		},
		{
			name:    "bad path nil values",
			wantErr: "values",
		},
		{
			name:   "bad path write option fails",
			values: map[string]interface{}{"KEY": "value"},
			opts: []option.WriteFunc{
				func(*option.Write) error {
					return errors.New("write option failed")
				},
			},
			wantErr: "write option failed",
		},
		{
			name:        "bad path patch error",
			values:      map[string]interface{}{"KEY": "value"},
			patchStatus: http.StatusInternalServerError,
			patchBody:   `{"message":"patch failed"}`,
			wantErr:     "patch Kubernetes secret",
			wantRequests: []capturedRequest{
				{
					method:      http.MethodPatch,
					path:        "/api/v1/namespaces/testing/secrets/application",
					contentType: "application/strategic-merge-patch+json",
					body:        `{"data":{"KEY":"dmFsdWU="}}`,
				},
			},
		},
		{
			name:         "bad path create error",
			values:       map[string]interface{}{"KEY": "value"},
			patchStatus:  http.StatusNotFound,
			createStatus: http.StatusConflict,
			createBody:   `{"message":"create failed"}`,
			wantErr:      "create Kubernetes secret",
			wantRequests: []capturedRequest{
				{
					method:      http.MethodPatch,
					path:        "/api/v1/namespaces/testing/secrets/application",
					contentType: "application/strategic-merge-patch+json",
					body:        `{"data":{"KEY":"dmFsdWU="}}`,
				},
				{
					method:      http.MethodPost,
					path:        "/api/v1/namespaces/testing/secrets",
					contentType: "application/json",
					body: `{
						"apiVersion":"v1",
						"kind":"Secret",
						"metadata":{"name":"application"},
						"data":{"KEY":"dmFsdWU="}
					}`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				requests []capturedRequest
				mutex    sync.Mutex
			)

			server, listener := newK8sSecretTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body interface{}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				bodyJSON, err := json.Marshal(body)
				require.NoError(t, err)

				mutex.Lock()
				requests = append(requests, capturedRequest{
					method:      r.Method,
					path:        r.URL.Path,
					contentType: r.Header.Get("Content-Type"),
					body:        string(bodyJSON),
				})
				mutex.Unlock()

				assert.Equal(t, "Bearer api-token", r.Header.Get("Authorization"))

				if r.Method == http.MethodPatch {
					w.WriteHeader(tt.patchStatus)
					_, _ = w.Write([]byte(tt.patchBody))

					return
				}

				w.WriteHeader(tt.createStatus)
				_, _ = w.Write([]byte(tt.createBody))
			}), false)

			created, err := New(false, false, &Config{
				APIServer:  server.URL,
				Namespace:  "testing",
				SecretName: "application",
				Token:      "api-token",
			})
			require.NoError(t, err)
			useK8sSecretTestServer(t, created, listener)

			err = created.Write(context.Background(), tt.values, tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			mutex.Lock()
			gotRequests := append([]capturedRequest(nil), requests...)
			mutex.Unlock()

			require.Len(t, gotRequests, len(tt.wantRequests))
			for index := range tt.wantRequests {
				assert.Equal(t, tt.wantRequests[index].method, gotRequests[index].method)
				assert.Equal(t, tt.wantRequests[index].path, gotRequests[index].path)
				assert.Equal(t, tt.wantRequests[index].contentType, gotRequests[index].contentType)
				assert.JSONEq(t, tt.wantRequests[index].body, gotRequests[index].body)
			}
		})
	}
}

func TestK8sSecretWriteRequestErrors(t *testing.T) {
	tests := []struct {
		name        string
		closeServer bool
		tokenFile   string
		wantErr     string
	}{
		{
			name:        "bad path patch request fails",
			closeServer: true,
			wantErr:     "patch Kubernetes secret",
		},
		{
			name:      "bad path token file is unavailable",
			tokenFile: filepath.Join(t.TempDir(), "missing-token"),
			wantErr:   "read Kubernetes token file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, listener := newK8sSecretTestServer(
				t,
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
				false,
			)

			config := &Config{
				APIServer:  server.URL,
				SecretName: "application",
				Token:      "api-token",
			}
			if tt.tokenFile != "" {
				config.Token = ""
				config.TokenFile = tt.tokenFile
			}

			created, err := New(false, false, config)
			require.NoError(t, err)
			useK8sSecretTestServer(t, created, listener)

			if tt.closeServer {
				server.Close()
			}

			err = created.Write(
				context.Background(),
				map[string]interface{}{"KEY": "value"},
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func newK8sSecretTestServer(
	t *testing.T,
	handler http.Handler,
	useTLS bool,
) (*httptest.Server, *pipeListener) {
	t.Helper()

	listener := newPipeListener()
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}

	if useTLS {
		server.StartTLS()
	} else {
		server.Start()
	}

	t.Cleanup(server.Close)

	return server, listener
}

func useK8sSecretTestServer(
	t *testing.T,
	created provider.IProvider,
	listener *pipeListener,
) {
	t.Helper()

	typed, ok := created.(*K8sSecret)
	require.True(t, ok)

	transport, ok := typed.client.Transport.(*http.Transport)
	require.True(t, ok)
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}

	t.Cleanup(transport.CloseIdleConnections)
}

type pipeListener struct {
	connections chan net.Conn
	closed      chan struct{}
	closeOnce   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{
		connections: make(chan net.Conn),
		closed:      make(chan struct{}),
	}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case connection := <-l.connections:
		return connection, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.closed)
	})

	return nil
}

func (l *pipeListener) Addr() net.Addr {
	return pipeAddress{}
}

func (l *pipeListener) DialContext(ctx context.Context) (net.Conn, error) {
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

type pipeAddress struct{}

func (pipeAddress) Network() string {
	return "pipe"
}

func (pipeAddress) String() string {
	return "example.com"
}
