package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/httpclient/v2"
	"golang.org/x/crypto/nacl/box"
)

//////
// Helpers.
//////

type observedRequest struct {
	method   string
	path     string
	secret   SecretRequest
	variable VariableRequest
}

type testServer struct {
	URL string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newPublicKey(t *testing.T) string {
	t.Helper()

	publicKey, _, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(publicKey[:])
}

func useGitHubAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := newHTTPTestServer(t, handler)
	originalBaseURL := githubAPIBaseURL
	githubAPIBaseURL = server.URL

	t.Cleanup(func() {
		githubAPIBaseURL = originalBaseURL
	})
}

func newHTTPTestServer(t *testing.T, handler http.Handler) *testServer {
	t.Helper()

	var server *httptest.Server

	func() {
		defer func() {
			_ = recover()
		}()

		server = httptest.NewServer(handler)
	}()

	if server != nil {
		t.Cleanup(server.Close)

		return &testServer{URL: server.URL}
	}

	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		response := recorder.Result()
		response.Request = request

		return response, nil
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	return &testServer{URL: "http://github-api.test"}
}

func publicKeyHandler(actionsKey, actionsKeyID, codespacesKey, codespacesKeyID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/secrets/public-key"):
			_ = json.NewEncoder(w).Encode(PublicKeyResponse{Key: actionsKey, KeyID: actionsKeyID})
		case strings.HasSuffix(r.URL.Path, "/codespaces/secrets/public-key"):
			_ = json.NewEncoder(w).Encode(PublicKeyResponse{Key: codespacesKey, KeyID: codespacesKeyID})
		default:
			http.NotFound(w, r)
		}
	}
}

func newTestGitHub(t *testing.T, handler http.HandlerFunc) *GitHub {
	t.Helper()

	actionsKey := newPublicKey(t)
	codespacesKey := newPublicKey(t)

	useGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/secrets/public-key"):
			_ = json.NewEncoder(w).Encode(PublicKeyResponse{Key: actionsKey, KeyID: "actions-key-id"})
		case strings.HasSuffix(r.URL.Path, "/codespaces/secrets/public-key"):
			_ = json.NewEncoder(w).Encode(PublicKeyResponse{Key: codespacesKey, KeyID: "codespaces-key-id"})
		default:
			handler(w, r)
		}
	})

	t.Setenv("GITHUB_TOKEN", "test-token")

	githubProvider, err := New(false, false, "octocat", "hello-world")
	require.NoError(t, err)

	return githubProvider
}

//////
// Encryption.
//////

func TestEncrypt(t *testing.T) {
	tests := []struct {
		name      string
		publicKey func(*testing.T) string
		secret    string
		wantErr   string
	}{
		{
			name:      "happy path",
			publicKey: newPublicKey,
			secret:    "sensitive value",
		},
		{
			name:      "edge case empty secret",
			publicKey: newPublicKey,
			secret:    "",
		},
		{
			name:      "bad path malformed base64",
			publicKey: func(*testing.T) string { return "not-base64%%" },
			secret:    "value",
			wantErr:   "decode public key",
		},
		{
			name: "bad path decoded key has wrong length",
			publicKey: func(*testing.T) string {
				return base64.StdEncoding.EncodeToString([]byte("short"))
			},
			secret:  "value",
			wantErr: "public key must decode to 32 bytes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encrypted, err := encrypt(test.publicKey(t), test.secret)

			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
				assert.Empty(t, encrypted)

				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, encrypted)

			decoded, err := base64.StdEncoding.DecodeString(encrypted)
			require.NoError(t, err)
			assert.Greater(t, len(decoded), len(test.secret))
		})
	}
}

//////
// Factory.
//////

func TestNewValidation(t *testing.T) {
	validKey := newPublicKey(t)

	tests := []struct {
		name    string
		token   string
		owner   string
		repo    string
		wantErr string
	}{
		{
			name:    "bad path missing token",
			owner:   "octocat",
			repo:    "hello-world",
			wantErr: "GITHUB_TOKEN",
		},
		{
			name:    "bad path missing owner",
			token:   "token",
			repo:    "hello-world",
			wantErr: "Owner",
		},
		{
			name:    "bad path missing repo",
			token:   "token",
			owner:   "octocat",
			wantErr: "Repo",
		},
		{
			name:  "happy path",
			token: "token",
			owner: "octocat",
			repo:  "hello-world",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useGitHubAPI(t, publicKeyHandler(
				validKey,
				"actions-key-id",
				validKey,
				"codespaces-key-id",
			))
			t.Setenv("GITHUB_TOKEN", test.token)

			got, err := New(false, false, test.owner, test.repo)

			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, test.owner, got.Owner)
			assert.Equal(t, test.repo, got.Repo)
			assert.Equal(t, test.token, got.Token)
			assert.Equal(t, "actions-key-id", got.publicKeyResponseActions.KeyID)
			assert.Equal(t, "codespaces-key-id", got.publicKeyResponseCodespace.KeyID)
			assert.Equal(t, "Bearer token", got.client.Headers["Authorization"])
		})
	}
}

func TestNewPublicKeyErrors(t *testing.T) {
	validKey := newPublicKey(t)

	tests := []struct {
		name       string
		failingKey Target
		status     int
	}{
		{name: "bad path actions key unauthorized", failingKey: Actions, status: http.StatusUnauthorized},
		{name: "bad path codespaces key not found", failingKey: Codespaces, status: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/"+test.failingKey.String()+"/") {
					http.Error(w, "key unavailable", test.status)

					return
				}

				_ = json.NewEncoder(w).Encode(PublicKeyResponse{
					Key:   validKey,
					KeyID: "valid-key-id",
				})
			})
			t.Setenv("GITHUB_TOKEN", "token")

			got, err := New(false, false, "octocat", "hello-world")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "publicKey information")
			assert.Nil(t, got)
		})
	}
}

//////
// Provider implementation.
//////

func TestLoad(t *testing.T) {
	githubProvider := &GitHub{}

	got, err := githubProvider.Load(context.Background(), option.WithKeyPrefixer("ignored-"))

	assert.Nil(t, got)
	assert.ErrorIs(t, err, provider.ErrNotSupported)
}

func TestWriteHappyPaths(t *testing.T) {
	tests := []struct {
		name      string
		target    Target
		wantKeyID string
	}{
		{name: "happy path actions secret", target: Actions, wantKeyID: "actions-key-id"},
		{name: "happy path codespaces secret", target: Codespaces, wantKeyID: "codespaces-key-id"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := make(chan observedRequest, 1)
			githubProvider := newTestGitHub(t, func(w http.ResponseWriter, r *http.Request) {
				var body SecretRequest
				_ = json.NewDecoder(r.Body).Decode(&body)
				requests <- observedRequest{
					method: r.Method,
					path:   r.URL.Path,
					secret: body,
				}
				w.WriteHeader(http.StatusCreated)
			})

			err := githubProvider.Write(
				context.Background(),
				map[string]interface{}{"API_TOKEN": "top-secret"},
				option.WithTarget(test.target.String()),
			)

			require.NoError(t, err)
			request := <-requests
			assert.Equal(t, http.MethodPut, request.method)
			assert.Equal(
				t,
				fmt.Sprintf("/repos/octocat/hello-world/%s/secrets/API_TOKEN", test.target),
				request.path,
			)
			assert.Equal(t, test.wantKeyID, request.secret.KeyID)
			assert.NotEmpty(t, request.secret.EncryptedValue)
		})
	}
}

func TestWriteOptions(t *testing.T) {
	tests := []struct {
		name       string
		options    []option.WriteFunc
		wantMethod string
		wantPath   string
		wantSecret bool
	}{
		{
			name:       "happy path repository variable uses post",
			options:    []option.WriteFunc{option.WithTarget(Actions.String()), option.WithVariable(true)},
			wantMethod: http.MethodPost,
			wantPath:   "/repos/octocat/hello-world/actions/variables",
		},
		{
			name: "edge case environment variable honors forced patch",
			options: []option.WriteFunc{
				option.WithTarget(Actions.String()),
				option.WithEnvironment("production"),
				option.WithVariable(true),
				option.WithHTTPVerb(http.MethodPatch),
			},
			wantMethod: http.MethodPatch,
			wantPath:   "/repositories/42/environments/production/variables/CONFIG",
		},
		{
			name: "happy path environment secret uses put",
			options: []option.WriteFunc{
				option.WithTarget(Actions.String()),
				option.WithEnvironment("production"),
			},
			wantMethod: http.MethodPut,
			wantPath:   "/repositories/42/environments/production/secrets/CONFIG",
			wantSecret: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := make(chan observedRequest, 1)
			githubProvider := newTestGitHub(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/repos/octocat/hello-world" {
					_ = json.NewEncoder(w).Encode(Repository{ID: 42, NodeID: "node-id"})

					return
				}

				request := observedRequest{method: r.Method, path: r.URL.Path}
				if strings.Contains(r.URL.Path, "/variables") {
					_ = json.NewDecoder(r.Body).Decode(&request.variable)
				} else {
					_ = json.NewDecoder(r.Body).Decode(&request.secret)
				}

				requests <- request
				w.WriteHeader(http.StatusNoContent)
			})

			err := githubProvider.Write(
				context.Background(),
				map[string]interface{}{"CONFIG": 123},
				test.options...,
			)

			require.NoError(t, err)
			request := <-requests
			assert.Equal(t, test.wantMethod, request.method)
			assert.Equal(t, test.wantPath, request.path)

			if test.wantSecret {
				assert.Equal(t, "actions-key-id", request.secret.KeyID)
				assert.NotEmpty(t, request.secret.EncryptedValue)
			} else {
				assert.Equal(t, VariableRequest{Name: "CONFIG", Value: "123"}, request.variable)
			}
		})
	}
}

func TestWriteErrorsAndEdges(t *testing.T) {
	optionErr := errors.New("option failed")

	tests := []struct {
		name       string
		values     map[string]interface{}
		options    []option.WriteFunc
		handler    http.HandlerFunc
		wantErr    string
		wantCalled bool
	}{
		{
			name:    "bad path nil values",
			values:  nil,
			wantErr: "values",
		},
		{
			name:   "edge case empty values",
			values: map[string]interface{}{},
		},
		{
			name:   "bad path option error",
			values: map[string]interface{}{"CONFIG": "value"},
			options: []option.WriteFunc{func(*option.Write) error {
				return optionErr
			}},
			wantErr: optionErr.Error(),
		},
		{
			name:       "bad path environment repository lookup fails",
			values:     map[string]interface{}{"CONFIG": "value"},
			options:    []option.WriteFunc{option.WithEnvironment("production")},
			handler:    func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "missing", http.StatusNotFound) },
			wantErr:    "failed to request",
			wantCalled: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var called atomic.Bool
			githubProvider := newTestGitHub(t, func(w http.ResponseWriter, r *http.Request) {
				called.Store(true)
				if test.handler != nil {
					test.handler(w, r)

					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			err := githubProvider.Write(context.Background(), test.values, test.options...)

			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.wantCalled, called.Load())
		})
	}
}

func TestWriteAPIErrorResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "bad path unauthorized", status: http.StatusUnauthorized},
		{name: "bad path not found", status: http.StatusNotFound},
		{name: "bad path validation rejected", status: http.StatusUnprocessableEntity},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			githubProvider := newTestGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, http.StatusText(test.status), test.status)
			})

			err := githubProvider.Write(
				context.Background(),
				map[string]interface{}{"CONFIG": "value"},
				option.WithTarget(Actions.String()),
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), http.StatusText(test.status))
		})
	}
}

func TestWriteInvalidPublicKey(t *testing.T) {
	validKey := newPublicKey(t)
	tests := []struct {
		name      string
		publicKey string
		wantErr   string
	}{
		{
			name:      "bad path malformed public key",
			publicKey: "not-base64%%",
			wantErr:   "decode public key",
		},
		{
			name:      "bad path short decoded public key",
			publicKey: base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr:   "public key must decode to 32 bytes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var called atomic.Bool
			useGitHubAPI(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/actions/secrets/public-key"):
					_ = json.NewEncoder(w).Encode(PublicKeyResponse{
						Key:   test.publicKey,
						KeyID: "actions-key-id",
					})
				case strings.HasSuffix(r.URL.Path, "/codespaces/secrets/public-key"):
					_ = json.NewEncoder(w).Encode(PublicKeyResponse{
						Key:   validKey,
						KeyID: "codespaces-key-id",
					})
				default:
					called.Store(true)
					w.WriteHeader(http.StatusNoContent)
				}
			})
			t.Setenv("GITHUB_TOKEN", "token")

			githubProvider, err := New(false, false, "octocat", "hello-world")
			require.NoError(t, err)

			err = githubProvider.Write(
				context.Background(),
				map[string]interface{}{"CONFIG": "value"},
				option.WithTarget(Actions.String()),
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
			assert.False(t, called.Load())
		})
	}
}

//////
// Request construction and API helpers.
//////

func TestConstructRequestDetails(t *testing.T) {
	originalBaseURL := githubAPIBaseURL
	githubAPIBaseURL = "https://github.test"
	t.Cleanup(func() { githubAPIBaseURL = originalBaseURL })

	githubProvider := &GitHub{Owner: "octocat", Repo: "hello-world"}
	variableRequest := &VariableRequest{Name: "CONFIG", Value: "value"}
	secretRequest := &SecretRequest{EncryptedValue: "encrypted", KeyID: "key-id"}

	tests := []struct {
		name       string
		options    option.Write
		repository *Repository
		forceVerb  string
		wantVerb   string
		wantURL    string
	}{
		{
			name:     "happy path repository secret",
			options:  option.Write{Target: Actions.String()},
			wantVerb: http.MethodPut,
			wantURL:  "https://github.test/repos/octocat/hello-world/actions/secrets/CONFIG",
		},
		{
			name:     "happy path repository variable",
			options:  option.Write{Target: Actions.String(), Variable: true},
			wantVerb: http.MethodPost,
			wantURL:  "https://github.test/repos/octocat/hello-world/actions/variables",
		},
		{
			name:       "happy path environment secret",
			options:    option.Write{Environment: "production", Target: Actions.String()},
			repository: &Repository{ID: 42},
			wantVerb:   http.MethodPut,
			wantURL:    "https://github.test/repositories/42/environments/production/secrets/CONFIG",
		},
		{
			name:       "happy path environment variable",
			options:    option.Write{Environment: "production", Target: Actions.String(), Variable: true},
			repository: &Repository{ID: 42},
			wantVerb:   http.MethodPost,
			wantURL:    "https://github.test/repositories/42/environments/production/variables",
		},
		{
			name:       "happy path forced environment variable verb",
			options:    option.Write{Environment: "production", Target: Actions.String(), Variable: true},
			repository: &Repository{ID: 42},
			forceVerb:  http.MethodPatch,
			wantVerb:   http.MethodPatch,
			wantURL:    "https://github.test/repositories/42/environments/production/variables/CONFIG",
		},
		{
			name:      "edge case forced variable verb with nil repository",
			options:   option.Write{Target: Actions.String(), Variable: true},
			forceVerb: http.MethodPatch,
			wantVerb:  http.MethodPatch,
			wantURL:   "https://github.test/repos/octocat/hello-world/actions/variables",
		},
		{
			name:      "edge case forced secret verb",
			options:   option.Write{Target: Codespaces.String()},
			forceVerb: http.MethodPatch,
			wantVerb:  http.MethodPatch,
			wantURL:   "https://github.test/repos/octocat/hello-world/codespaces/secrets/CONFIG",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verb, url, requestBody := githubProvider.constructRequestDetails(
				test.options,
				test.repository,
				"CONFIG",
				variableRequest,
				secretRequest,
				test.forceVerb,
			)

			assert.Equal(t, test.wantVerb, verb)
			assert.Equal(t, test.wantURL, url)
			assert.NotNil(t, requestBody)
		})
	}
}

func TestGetRepository(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		want    *Repository
		wantErr bool
	}{
		{
			name:   "happy path",
			status: http.StatusOK,
			want:   &Repository{ID: 42, NodeID: "node-id"},
		},
		{name: "bad path unauthorized", status: http.StatusUnauthorized, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			githubProvider := newTestGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				if test.status == http.StatusOK {
					_ = json.NewEncoder(w).Encode(test.want)
				}
			})

			got, err := githubProvider.GetRepository(context.Background())

			if test.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestList(t *testing.T) {
	createdAt := time.Date(2026, time.July, 29, 1, 2, 3, 0, time.UTC)
	want := &SecretsResponse{
		TotalCount: 1,
		Secrets: []SecretsResponseSecret{{
			Name:      "CONFIG",
			CreatedAt: createdAt,
			UpdatedAt: createdAt.Add(time.Hour),
		}},
	}

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "happy path", status: http.StatusOK},
		{name: "bad path unauthorized", status: http.StatusUnauthorized, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			githubProvider := newTestGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				if test.status == http.StatusOK {
					_ = json.NewEncoder(w).Encode(want)
				}
			})

			got, err := List(context.Background(), githubProvider)

			if test.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name       string
		secrets    []string
		status     int
		wantErr    bool
		wantCalled bool
	}{
		{
			name:       "happy path",
			secrets:    []string{"CONFIG"},
			status:     http.StatusNoContent,
			wantCalled: true,
		},
		{
			name:       "bad path unauthorized",
			secrets:    []string{"CONFIG"},
			status:     http.StatusUnauthorized,
			wantErr:    true,
			wantCalled: true,
		},
		{
			name:    "edge case empty secrets",
			secrets: []string{},
			status:  http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var called atomic.Bool
			githubProvider := newTestGitHub(t, func(w http.ResponseWriter, r *http.Request) {
				called.Store(true)
				assert.Equal(t, http.MethodDelete, r.Method)
				assert.Equal(t, "/repos/octocat/hello-world/actions/secrets/CONFIG", r.URL.Path)
				w.WriteHeader(test.status)
			})

			err := Delete(context.Background(), githubProvider, test.secrets...)

			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.wantCalled, called.Load())
		})
	}
}

func TestExecuteRequestMethods(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		status  int
		wantErr bool
	}{
		{name: "post happy path", method: http.MethodPost, status: http.StatusCreated},
		{name: "post bad path", method: http.MethodPost, status: http.StatusUnauthorized, wantErr: true},
		{name: "put happy path", method: http.MethodPut, status: http.StatusNoContent},
		{name: "put bad path", method: http.MethodPut, status: http.StatusNotFound, wantErr: true},
		{name: "patch happy path", method: http.MethodPatch, status: http.StatusOK},
		{name: "patch bad path", method: http.MethodPatch, status: http.StatusUnprocessableEntity, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, test.method, r.Method)
				w.WriteHeader(test.status)
			}))

			client, err := httpclient.NewDefault(httpclient.WithClientName("github-execute-test"))
			require.NoError(t, err)
			githubProvider := &GitHub{client: client}

			var ok bool
			switch test.method {
			case http.MethodPost:
				ok, err = githubProvider.executePOSTRequest(
					context.Background(),
					server.URL,
					httpclient.WithReqBody(map[string]string{"key": "value"}),
				)
			case http.MethodPut:
				ok, err = githubProvider.executePUTRequest(
					context.Background(),
					server.URL,
					httpclient.WithReqBody(map[string]string{"key": "value"}),
				)
			case http.MethodPatch:
				ok, err = githubProvider.executePATCHRequest(
					context.Background(),
					server.URL,
					httpclient.WithReqBody(map[string]string{"key": "value"}),
				)
			}

			if test.wantErr {
				require.Error(t, err)
				assert.False(t, ok)

				return
			}

			require.NoError(t, err)
			assert.True(t, ok)
		})
	}
}
