package onepassword

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

const (
	testVaultID = "aaaaaaaaaaaaaaaaaaaaaaaaaa"
	testItemID  = "bbbbbbbbbbbbbbbbbbbbbbbbbb"
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
			name: "happy path",
			config: &Config{
				Host:  "https://connect.example.test",
				Token: "token",
				Vault: testVaultID,
				Item:  testItemID,
			},
		},
		{
			name:    "bad path nil config",
			wantErr: "config",
		},
		{
			name: "bad path missing host",
			config: &Config{
				Token: "token",
				Vault: testVaultID,
				Item:  testItemID,
			},
			wantErr: "Host",
		},
		{
			name: "bad path missing token",
			config: &Config{
				Host:  "https://connect.example.test",
				Vault: testVaultID,
				Item:  testItemID,
			},
			wantErr: "Token",
		},
		{
			name: "bad path missing vault",
			config: &Config{
				Host:  "https://connect.example.test",
				Token: "token",
				Item:  testItemID,
			},
			wantErr: "Vault",
		},
		{
			name: "bad path missing item",
			config: &Config{
				Host:  "https://connect.example.test",
				Token: "token",
				Vault: testVaultID,
			},
			wantErr: "Item",
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

func TestOnePasswordLoad(t *testing.T) {
	tests := []struct {
		name         string
		vault        string
		item         string
		override     bool
		rawValue     bool
		existingEnv  map[string]string
		opts         []option.LoadKeyFunc
		handler      http.HandlerFunc
		want         map[string]string
		wantErr      string
		wantRequests int32
	}{
		{
			name:  "happy path resolves names extracts fields and applies options",
			vault: "Production Vault",
			item:  "Application",
			opts: []option.LoadKeyFunc{
				option.WithKeyCaser("upper"),
				option.WithKeyPrefixer("OP_"),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
				switch r.URL.Path {
				case "/v1/vaults":
					assert.Equal(t, `name eq "Production Vault"`, r.URL.Query().Get("filter"))
					writeJSON(t, w, http.StatusOK, []map[string]string{{"id": testVaultID}})
				case "/v1/vaults/" + testVaultID + "/items":
					assert.Equal(t, `title eq "Application"`, r.URL.Query().Get("filter"))
					writeJSON(t, w, http.StatusOK, []map[string]string{{"id": testItemID}})
				case "/v1/vaults/" + testVaultID + "/items/" + testItemID:
					writeJSON(t, w, http.StatusOK, map[string]interface{}{
						"fields": []map[string]string{
							{"id": "username", "label": "api_key", "value": "secret"},
							{"id": "fallback_id", "value": "fallback"},
							{"id": "", "label": "", "value": "skip-no-key"},
							{"id": "skip_empty", "label": "EMPTY", "value": ""},
						},
						"sections": []map[string]string{{"id": "ignored"}},
					})
				default:
					http.NotFound(w, r)
				}
			},
			want: map[string]string{
				"OP_API_KEY":     "secret",
				"OP_FALLBACK_ID": "fallback",
			},
			wantRequests: 3,
		},
		{
			name:        "edge UUIDs skip resolution and preserve existing environment",
			vault:       testVaultID,
			item:        testItemID,
			existingEnv: map[string]string{"OP_EXISTING": "existing"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/vaults/"+testVaultID+"/items/"+testItemID, r.URL.Path)
				writeJSON(t, w, http.StatusOK, map[string]interface{}{
					"fields": []map[string]string{{"label": "OP_EXISTING", "value": "loaded"}},
				})
			},
			want:         map[string]string{"OP_EXISTING": "existing"},
			wantRequests: 1,
		},
		{
			name:     "edge override and raw value",
			vault:    testVaultID,
			item:     testItemID,
			override: true,
			rawValue: true,
			existingEnv: map[string]string{
				"OP_RAW": "existing",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, map[string]interface{}{
					"fields": []map[string]string{{"label": "OP_RAW", "value": "line\nvalue"}},
				})
			},
			want:         map[string]string{"OP_RAW": `"line\nvalue"`},
			wantRequests: 1,
		},
		{
			name:  "edge empty fields returns empty map",
			vault: testVaultID,
			item:  testItemID,
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, map[string]interface{}{"fields": []interface{}{}})
			},
			want:         map[string]string{},
			wantRequests: 1,
		},
		{
			name:  "bad path vault name not found",
			vault: "Missing Vault",
			item:  testItemID,
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, []interface{}{})
			},
			wantErr:      "Missing Vault",
			wantRequests: 1,
		},
		{
			name:  "bad path vault name is ambiguous",
			vault: "Duplicate Vault",
			item:  testItemID,
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, []map[string]string{
					{"id": testVaultID},
					{"id": "cccccccccccccccccccccccccc"},
				})
			},
			wantErr:      "ambiguous",
			wantRequests: 1,
		},
		{
			name:  "bad path item name not found",
			vault: testVaultID,
			item:  "Missing Item",
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, []interface{}{})
			},
			wantErr:      "Missing Item",
			wantRequests: 1,
		},
		{
			name:  "bad path item name is ambiguous",
			vault: testVaultID,
			item:  "Duplicate Item",
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, []map[string]string{
					{"id": testItemID},
					{"id": "dddddddddddddddddddddddddd"},
				})
			},
			wantErr:      "ambiguous",
			wantRequests: 1,
		},
		{
			name:  "bad path unauthorized",
			vault: testVaultID,
			item:  testItemID,
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusUnauthorized, map[string]string{"message": "invalid token"})
			},
			wantErr:      "401",
			wantRequests: 1,
		},
		{
			name:  "bad path missing item UUID",
			vault: testVaultID,
			item:  testItemID,
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
			},
			wantErr:      "404",
			wantRequests: 1,
		},
		{
			name:  "bad path server error",
			vault: testVaultID,
			item:  testItemID,
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusInternalServerError, map[string]string{"message": "failure"})
			},
			wantErr:      "500",
			wantRequests: 1,
		},
		{
			name:  "bad path malformed JSON",
			vault: testVaultID,
			item:  testItemID,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"fields":`))
			},
			wantErr:      "decode 1Password response",
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.existingEnv {
				t.Setenv(key, value)
			}

			var requests atomic.Int32
			server, listener := newOnePasswordTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
				tt.handler(w, r)
			}))

			created, err := New(tt.override, tt.rawValue, &Config{
				Host:  server.URL + "/",
				Token: "token",
				Vault: tt.vault,
				Item:  tt.item,
			})
			require.NoError(t, err)
			useOnePasswordTestServer(t, created, listener)

			got, err := created.Load(context.Background(), tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				for key, value := range tt.want {
					assert.Equal(t, value, os.Getenv(key))
				}
			}

			assert.Equal(t, tt.wantRequests, requests.Load())
		})
	}
}

//////
// Write.
//////

func TestOnePasswordWrite(t *testing.T) {
	tests := []struct {
		name         string
		vault        string
		item         string
		values       map[string]interface{}
		opts         []option.WriteFunc
		handler      http.HandlerFunc
		wantErr      string
		wantRequests int32
	}{
		{
			name:   "happy path updates full item and merges fields",
			vault:  testVaultID,
			item:   testItemID,
			values: map[string]interface{}{"EXISTING": "updated", "NEW": 42},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
				switch r.Method {
				case http.MethodGet:
					writeJSON(t, w, http.StatusOK, map[string]interface{}{
						"id":       testItemID,
						"title":    "Application",
						"category": secureNoteCategory,
						"favorite": true,
						"fields": []map[string]interface{}{
							{
								"id":      "existing-id",
								"label":   "EXISTING",
								"type":    "STRING",
								"value":   "old",
								"purpose": "USERNAME",
							},
							{
								"id":    "preserved-id",
								"label": "PRESERVED",
								"type":  "STRING",
								"value": "preserved",
							},
						},
					})
				case http.MethodPut:
					assert.Equal(t, "/v1/vaults/"+testVaultID+"/items/"+testItemID, r.URL.Path)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					var body map[string]interface{}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.Equal(t, true, body["favorite"])

					fields := fieldsByLabel(t, body)
					assert.Equal(t, "updated", fields["EXISTING"]["value"])
					assert.Equal(t, concealedFieldType, fields["EXISTING"]["type"])
					assert.Equal(t, "USERNAME", fields["EXISTING"]["purpose"])
					assert.Equal(t, "preserved", fields["PRESERVED"]["value"])
					assert.Equal(t, "42", fields["NEW"]["value"])
					assert.Equal(t, concealedFieldType, fields["NEW"]["type"])
					w.WriteHeader(http.StatusOK)
				default:
					t.Fatalf("unexpected method %s", r.Method)
				}
			},
			wantRequests: 2,
		},
		{
			name:   "happy path creates secure note when item name is not found",
			vault:  testVaultID,
			item:   "New Application",
			values: map[string]interface{}{"B": "second", "A": "first"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					assert.Equal(t, `title eq "New Application"`, r.URL.Query().Get("filter"))
					writeJSON(t, w, http.StatusOK, []interface{}{})
				case http.MethodPost:
					assert.Equal(t, "/v1/vaults/"+testVaultID+"/items", r.URL.Path)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					var body map[string]interface{}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.Equal(t, "New Application", body["title"])
					assert.Equal(t, secureNoteCategory, body["category"])
					vault, ok := body["vault"].(map[string]interface{})
					require.True(t, ok)
					assert.Equal(t, testVaultID, vault["id"])

					fields, ok := body["fields"].([]interface{})
					require.True(t, ok)
					require.Len(t, fields, 2)

					fieldA, ok := fields[0].(map[string]interface{})
					require.True(t, ok)
					assert.Equal(t, "A", fieldA["label"])
					assert.Equal(t, concealedFieldType, fieldA["type"])

					fieldB, ok := fields[1].(map[string]interface{})
					require.True(t, ok)
					assert.Equal(t, "B", fieldB["label"])
					w.WriteHeader(http.StatusCreated)
				default:
					t.Fatalf("unexpected method %s", r.Method)
				}
			},
			wantRequests: 2,
		},
		{
			name:   "edge UUID returning 404 creates item",
			vault:  testVaultID,
			item:   testItemID,
			values: map[string]interface{}{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
				case http.MethodPost:
					var body map[string]interface{}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.Equal(t, testItemID, body["title"])
					assert.Empty(t, body["fields"])
					w.WriteHeader(http.StatusCreated)
				default:
					t.Fatalf("unexpected method %s", r.Method)
				}
			},
			wantRequests: 2,
		},
		{
			name:   "edge item without fields adds values",
			vault:  testVaultID,
			item:   testItemID,
			values: map[string]interface{}{"KEY": "value"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					writeJSON(t, w, http.StatusOK, map[string]interface{}{"id": testItemID})

					return
				}

				var body map[string]interface{}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				fields, ok := body["fields"].([]interface{})
				require.True(t, ok)
				require.Len(t, fields, 1)

				field, ok := fields[0].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "KEY", field["label"])
				w.WriteHeader(http.StatusOK)
			},
			wantRequests: 2,
		},
		{
			name:    "bad path nil values",
			vault:   testVaultID,
			item:    testItemID,
			wantErr: "values",
		},
		{
			name:   "bad path write option fails",
			vault:  testVaultID,
			item:   testItemID,
			values: map[string]interface{}{"KEY": "value"},
			opts: []option.WriteFunc{
				func(*option.Write) error {
					return errors.New("write option failed")
				},
			},
			wantErr: "write option failed",
		},
		{
			name:   "bad path update API error",
			vault:  testVaultID,
			item:   testItemID,
			values: map[string]interface{}{"KEY": "value"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					writeJSON(t, w, http.StatusOK, map[string]interface{}{"fields": []interface{}{}})

					return
				}
				writeJSON(t, w, http.StatusInternalServerError, map[string]string{"message": "failure"})
			},
			wantErr:      "update 1Password item",
			wantRequests: 2,
		},
		{
			name:   "bad path create API error",
			vault:  testVaultID,
			item:   "New Item",
			values: map[string]interface{}{"KEY": "value"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					writeJSON(t, w, http.StatusOK, []interface{}{})

					return
				}
				writeJSON(t, w, http.StatusBadRequest, map[string]string{"message": "invalid"})
			},
			wantErr:      "create 1Password item",
			wantRequests: 2,
		},
		{
			name:   "bad path malformed item fields",
			vault:  testVaultID,
			item:   testItemID,
			values: map[string]interface{}{"KEY": "value"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, map[string]interface{}{"fields": "invalid"})
			},
			wantErr:      "fields must be an array",
			wantRequests: 1,
		},
		{
			name:   "bad path malformed field entry",
			vault:  testVaultID,
			item:   testItemID,
			values: map[string]interface{}{"KEY": "value"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, map[string]interface{}{"fields": []interface{}{"invalid"}})
			},
			wantErr:      "field must be an object",
			wantRequests: 1,
		},
		{
			name:   "bad path load-before-update API error",
			vault:  testVaultID,
			item:   testItemID,
			values: map[string]interface{}{"KEY": "value"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusUnauthorized, map[string]string{"message": "invalid token"})
			},
			wantErr:      "load 1Password item for update",
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			handler := tt.handler
			if handler == nil {
				handler = func(w http.ResponseWriter, r *http.Request) {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}

			server, listener := newOnePasswordTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
				handler(w, r)
			}))

			created, err := New(false, false, &Config{
				Host:  server.URL,
				Token: "token",
				Vault: tt.vault,
				Item:  tt.item,
			})
			require.NoError(t, err)
			useOnePasswordTestServer(t, created, listener)

			err = created.Write(context.Background(), tt.values, tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantRequests, requests.Load())
		})
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, statusCode int, value interface{}) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	require.NoError(t, json.NewEncoder(w).Encode(value))
}

func fieldsByLabel(t *testing.T, item map[string]interface{}) map[string]map[string]interface{} {
	t.Helper()

	result := make(map[string]map[string]interface{})
	fields, ok := item["fields"].([]interface{})
	require.True(t, ok)

	for _, rawField := range fields {
		field, ok := rawField.(map[string]interface{})
		require.True(t, ok)
		label, ok := field["label"].(string)
		require.True(t, ok)
		result[label] = field
	}

	return result
}

func newOnePasswordTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *pipeListener) {
	t.Helper()

	listener := newPipeListener()
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	server.URL = "http://onepassword.test"

	t.Cleanup(server.Close)

	return server, listener
}

func useOnePasswordTestServer(t *testing.T, created interface{}, listener *pipeListener) {
	t.Helper()

	onePassword, ok := created.(*OnePassword)
	require.True(t, ok)

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		},
	}
	onePassword.client.Transport = transport

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
	return "onepassword.test"
}
