package doppler

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/option"
)

//////
// Constructor.
//////

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr string
	}{
		{
			name: "happy path regular token",
			config: &Config{
				Token:   "dp.pt.token",
				Project: "project",
				Config:  "development",
			},
		},
		{
			name: "edge service token without project or config",
			config: &Config{
				Token: "dp.st.token",
			},
		},
		{
			name: "edge service token with project only",
			config: &Config{
				Token:   "dp.st.token",
				Project: "project",
			},
		},
		{
			name:    "bad path nil config",
			wantErr: "config",
		},
		{
			name: "bad path missing token",
			config: &Config{
				Project: "project",
				Config:  "development",
			},
			wantErr: "Token",
		},
		{
			name: "bad path regular token missing project",
			config: &Config{
				Token:  "dp.pt.token",
				Config: "development",
			},
			wantErr: "project and config",
		},
		{
			name: "bad path regular token missing config",
			config: &Config{
				Token:   "dp.pt.token",
				Project: "project",
			},
			wantErr: "project and config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(false, false, tt.config)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, Name, got.GetName())
		})
	}
}

//////
// Load.
//////

func TestDopplerLoad(t *testing.T) {
	tests := []struct {
		name          string
		override      bool
		rawValue      bool
		token         string
		project       string
		config        string
		responseCode  int
		responseBody  string
		existingEnv   map[string]string
		opts          []option.LoadKeyFunc
		want          map[string]string
		wantErr       string
		wantQuery     map[string]string
		wantAuthToken string
	}{
		{
			name:         "happy path downloads exports and applies key funcs",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusOK,
			responseBody: `{"api_key":"secret","PORT":8080}`,
			opts: []option.LoadKeyFunc{
				option.WithKeyCaser("upper"),
				option.WithKeyPrefixer("DOPPLER_TEST_"),
			},
			want: map[string]string{
				"DOPPLER_TEST_API_KEY": "secret",
				"DOPPLER_TEST_PORT":    "8080",
			},
			wantQuery: map[string]string{
				"format":  "json",
				"project": "project",
				"config":  "development",
			},
			wantAuthToken: "Bearer dp.pt.token",
		},
		{
			name:         "edge existing environment wins without override",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusOK,
			responseBody: `{"DOPPLER_TEST_OVERRIDE":"downloaded"}`,
			existingEnv: map[string]string{
				"DOPPLER_TEST_OVERRIDE": "existing",
			},
			want: map[string]string{
				"DOPPLER_TEST_OVERRIDE": "existing",
			},
		},
		{
			name:         "edge override replaces existing environment",
			override:     true,
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusOK,
			responseBody: `{"DOPPLER_TEST_OVERRIDE_ENABLED":"downloaded"}`,
			existingEnv: map[string]string{
				"DOPPLER_TEST_OVERRIDE_ENABLED": "existing",
			},
			want: map[string]string{
				"DOPPLER_TEST_OVERRIDE_ENABLED": "downloaded",
			},
		},
		{
			name:         "edge raw value skips string parsing",
			rawValue:     true,
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusOK,
			responseBody: `{"DOPPLER_TEST_RAW":"line\\nvalue"}`,
			want: map[string]string{
				"DOPPLER_TEST_RAW": `"line\\nvalue"`,
			},
		},
		{
			name:         "edge empty response returns empty map",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusOK,
			responseBody: `{}`,
			want:         map[string]string{},
		},
		{
			name:         "edge service token omits unset project and config query",
			token:        "dp.st.token",
			responseCode: http.StatusOK,
			responseBody: `{"DOPPLER_TEST_SERVICE":"value"}`,
			want: map[string]string{
				"DOPPLER_TEST_SERVICE": "value",
			},
			wantQuery: map[string]string{
				"format": "json",
			},
		},
		{
			name:         "bad path missing secret",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusNotFound,
			responseBody: `{"messages":["secret not found"]}`,
			wantErr:      "download secrets",
		},
		{
			name:         "bad path API error",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusUnauthorized,
			responseBody: `{"messages":["invalid token"]}`,
			wantErr:      "download secrets",
		},
		{
			name:         "bad path malformed payload",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusOK,
			responseBody: `[{"KEY":"value"}]`,
			wantErr:      "download secrets",
		},
		{
			name:         "bad path null payload is not an object",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			responseCode: http.StatusOK,
			responseBody: `null`,
			wantErr:      "JSON object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.existingEnv {
				t.Setenv(key, value)
			}

			server, listener := newDopplerTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/v3/configs/config/secrets/download", r.URL.Path)

				if tt.wantAuthToken != "" {
					assert.Equal(t, tt.wantAuthToken, r.Header.Get("Authorization"))
				} else {
					assert.Equal(t, "Bearer "+tt.token, r.Header.Get("Authorization"))
				}

				wantQuery := tt.wantQuery
				if wantQuery == nil {
					wantQuery = map[string]string{
						"format":  "json",
						"project": tt.project,
						"config":  tt.config,
					}
				}

				require.Len(t, r.URL.Query(), len(wantQuery))
				for key, value := range wantQuery {
					assert.Equal(t, value, r.URL.Query().Get(key))
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.responseCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))

			setDopplerAPIBaseURL(t, server.URL)

			created, err := New(tt.override, tt.rawValue, &Config{
				Token:   tt.token,
				Project: tt.project,
				Config:  tt.config,
			})
			require.NoError(t, err)

			typed, ok := created.(*Doppler)
			require.True(t, ok)
			useDopplerTestServer(t, typed, listener)

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

//////
// Write.
//////

func TestDopplerWrite(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		project      string
		config       string
		values       map[string]interface{}
		opts         []option.WriteFunc
		responseCode int
		responseBody string
		wantBody     map[string]interface{}
		wantErr      string
		wantRequests int
	}{
		{
			name:    "happy path writes secrets",
			token:   "dp.pt.token",
			project: "project",
			config:  "development",
			values: map[string]interface{}{
				"KEY":  "value",
				"PORT": float64(8080),
			},
			responseCode: http.StatusOK,
			responseBody: `{}`,
			wantBody: map[string]interface{}{
				"project": "project",
				"config":  "development",
				"secrets": map[string]interface{}{
					"KEY":  "value",
					"PORT": float64(8080),
				},
			},
			wantRequests: 1,
		},
		{
			name:         "edge writes empty secret map",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			values:       map[string]interface{}{},
			responseCode: http.StatusOK,
			responseBody: `{}`,
			wantBody: map[string]interface{}{
				"project": "project",
				"config":  "development",
				"secrets": map[string]interface{}{},
			},
			wantRequests: 1,
		},
		{
			name:         "edge service token sends optional values when set",
			token:        "dp.st.token",
			project:      "project",
			config:       "development",
			values:       map[string]interface{}{"KEY": "value"},
			responseCode: http.StatusOK,
			responseBody: `{}`,
			wantBody: map[string]interface{}{
				"project": "project",
				"config":  "development",
				"secrets": map[string]interface{}{"KEY": "value"},
			},
			wantRequests: 1,
		},
		{
			name:         "edge service token sends empty optional fields",
			token:        "dp.st.token",
			values:       map[string]interface{}{"KEY": "value"},
			responseCode: http.StatusOK,
			responseBody: `{}`,
			wantBody: map[string]interface{}{
				"project": "",
				"config":  "",
				"secrets": map[string]interface{}{"KEY": "value"},
			},
			wantRequests: 1,
		},
		{
			name:         "bad path nil values",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			wantErr:      "values",
			wantRequests: 0,
		},
		{
			name:    "bad path write option fails",
			token:   "dp.pt.token",
			project: "project",
			config:  "development",
			values:  map[string]interface{}{"KEY": "value"},
			opts: []option.WriteFunc{
				func(*option.Write) error {
					return errors.New("write option failed")
				},
			},
			wantErr:      "write option failed",
			wantRequests: 0,
		},
		{
			name:         "bad path API error",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			values:       map[string]interface{}{"KEY": "value"},
			responseCode: http.StatusBadRequest,
			responseBody: `{"messages":["invalid secret"]}`,
			wantBody: map[string]interface{}{
				"project": "project",
				"config":  "development",
				"secrets": map[string]interface{}{"KEY": "value"},
			},
			wantErr:      "write secrets",
			wantRequests: 1,
		},
		{
			name:         "bad path malformed request value",
			token:        "dp.pt.token",
			project:      "project",
			config:       "development",
			values:       map[string]interface{}{"KEY": make(chan int)},
			wantErr:      "write secrets",
			wantRequests: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			server, listener := newDopplerTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/v3/configs/config/secrets", r.URL.Path)
				assert.Equal(t, "Bearer "+tt.token, r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var gotBody map[string]interface{}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
				assert.Equal(t, tt.wantBody, gotBody)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.responseCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))

			setDopplerAPIBaseURL(t, server.URL)

			created, err := New(false, false, &Config{
				Token:   tt.token,
				Project: tt.project,
				Config:  tt.config,
			})
			require.NoError(t, err)

			typed, ok := created.(*Doppler)
			require.True(t, ok)
			useDopplerTestServer(t, typed, listener)

			err = created.Write(context.Background(), tt.values, tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, int32(tt.wantRequests), requests.Load())
		})
	}
}

func setDopplerAPIBaseURL(t *testing.T, baseURL string) {
	t.Helper()

	previous := dopplerAPIBaseURL
	dopplerAPIBaseURL = baseURL

	t.Cleanup(func() {
		dopplerAPIBaseURL = previous
	})
}

func newDopplerTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *pipeListener) {
	t.Helper()

	listener := newPipeListener()
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	server.URL = "http://doppler.test"

	t.Cleanup(server.Close)

	return server, listener
}

func useDopplerTestServer(t *testing.T, doppler *Doppler, listener *pipeListener) {
	t.Helper()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		},
	}
	doppler.client.GetClient().Transport = transport

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
	return "doppler.test"
}
