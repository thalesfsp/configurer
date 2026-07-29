package gcpsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	configureroption "github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//////
// Test client.
//////

type fakeSMClient struct {
	accessResponses map[string]*secretmanagerpb.AccessSecretVersionResponse
	accessErrors    map[string]error
	addErrors       []error
	addRequests     []*secretmanagerpb.AddSecretVersionRequest
	createErr       error
	createRequests  []*secretmanagerpb.CreateSecretRequest
}

func (f *fakeSMClient) AccessSecretVersion(
	_ context.Context,
	req *secretmanagerpb.AccessSecretVersionRequest,
	_ ...gax.CallOption,
) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if err := f.accessErrors[req.GetName()]; err != nil {
		return nil, err
	}

	return f.accessResponses[req.GetName()], nil
}

func (f *fakeSMClient) AddSecretVersion(
	_ context.Context,
	req *secretmanagerpb.AddSecretVersionRequest,
	_ ...gax.CallOption,
) (*secretmanagerpb.SecretVersion, error) {
	f.addRequests = append(f.addRequests, req)
	index := len(f.addRequests) - 1

	if index < len(f.addErrors) && f.addErrors[index] != nil {
		return nil, f.addErrors[index]
	}

	return &secretmanagerpb.SecretVersion{Name: req.GetParent() + "/versions/1"}, nil
}

func (f *fakeSMClient) CreateSecret(
	_ context.Context,
	req *secretmanagerpb.CreateSecretRequest,
	_ ...gax.CallOption,
) (*secretmanagerpb.Secret, error) {
	f.createRequests = append(f.createRequests, req)
	if f.createErr != nil {
		return nil, f.createErr
	}

	return req.GetSecret(), nil
}

func newTestGCPSM(
	t *testing.T,
	client secretManagerClient,
	override, rawValue bool,
	secretNames ...string,
) *GCPSM {
	t.Helper()

	baseProvider, err := provider.New(Name, override, rawValue)
	require.NoError(t, err)

	return &GCPSM{
		client:            client,
		Provider:          baseProvider,
		Config:            &Config{ProjectID: "test-project"},
		SecretInformation: &SecretInformation{SecretNames: secretNames},
	}
}

func secretResponse(payload []byte) *secretmanagerpb.AccessSecretVersionResponse {
	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{Data: payload},
	}
}

//////
// Constructors.
//////

func TestNew(t *testing.T) {
	originalNewSMClient := newSMClient
	t.Cleanup(func() {
		newSMClient = originalNewSMClient
	})

	clientErr := errors.New("client unavailable")

	tests := []struct {
		name              string
		config            *Config
		secretInformation *SecretInformation
		clientErr         error
		useNew            bool
		wantErr           string
	}{
		{
			name:              "happy path New",
			config:            &Config{ProjectID: "project"},
			secretInformation: &SecretInformation{SecretNames: []string{"secret"}},
			useNew:            true,
		},
		{
			name:              "happy path NewWithConfig",
			config:            &Config{ProjectID: "project"},
			secretInformation: &SecretInformation{SecretNames: []string{"secret"}},
		},
		{
			name:              "bad path missing config",
			secretInformation: &SecretInformation{SecretNames: []string{"secret"}},
			wantErr:           "config required",
		},
		{
			name:    "bad path missing secret information",
			config:  &Config{ProjectID: "project"},
			wantErr: "secret information required",
		},
		{
			name:              "bad path empty project",
			config:            &Config{},
			secretInformation: &SecretInformation{SecretNames: []string{"secret"}},
			wantErr:           "invalid struct",
		},
		{
			name:              "edge empty secret names",
			config:            &Config{ProjectID: "project"},
			secretInformation: &SecretInformation{},
			wantErr:           "invalid struct",
		},
		{
			name:              "edge blank secret name",
			config:            &Config{ProjectID: "project"},
			secretInformation: &SecretInformation{SecretNames: []string{""}},
			wantErr:           "invalid struct",
		},
		{
			name:              "bad path client initialization",
			config:            &Config{ProjectID: "project"},
			secretInformation: &SecretInformation{SecretNames: []string{"secret"}},
			clientErr:         clientErr,
			wantErr:           "initialize GCP Secret Manager client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakeSMClient{}
			clientCalls := 0
			optionCount := 0
			newSMClient = func(
				_ context.Context,
				opts ...option.ClientOption,
			) (secretManagerClient, error) {
				clientCalls++
				optionCount = len(opts)

				return fakeClient, tt.clientErr
			}

			var (
				got provider.IProvider
				err error
			)

			if tt.useNew {
				got, err = New(true, true, tt.config, tt.secretInformation)
			} else {
				got, err = NewWithConfig(
					true,
					true,
					tt.config,
					tt.secretInformation,
					option.WithEndpoint("localhost:1234"),
				)
			}

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.IsType(t, &GCPSM{}, got)
			assert.Equal(t, 1, clientCalls)
			if tt.useNew {
				assert.Zero(t, optionCount)
			} else {
				assert.Equal(t, 1, optionCount)
			}
			assert.Equal(t, Name, got.GetName())
			assert.True(t, got.GetOverride())
			assert.True(t, got.GetRawValue())
		})
	}
}

//////
// Load.
//////

func TestLoad(t *testing.T) {
	const (
		jsonSecretName  = "json-secret"
		plainSecretName = "folder/plain-secret"
	)

	jsonResource := "projects/test-project/secrets/json-secret/versions/latest"
	plainResource := "projects/test-project/secrets/folder/plain-secret/versions/latest"

	tests := []struct {
		name         string
		override     bool
		rawValue     bool
		secretNames  []string
		responses    map[string]*secretmanagerpb.AccessSecretVersionResponse
		accessErrors map[string]error
		setupEnv     map[string]string
		opts         []configureroption.LoadKeyFunc
		want         map[string]string
		wantErr      string
	}{
		{
			name:        "happy path JSON object",
			secretNames: []string{jsonSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				jsonResource: secretResponse([]byte(`{"USER":"admin","PORT":5432}`)),
			},
			want: map[string]string{"USER": "admin", "PORT": "5432"},
		},
		{
			name:        "happy path double encoded JSON with key functions",
			secretNames: []string{jsonSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				jsonResource: secretResponse([]byte(`"{\"user\":\"admin\"}"`)),
			},
			opts: []configureroption.LoadKeyFunc{
				configureroption.WithKeyCaser("upper"),
				configureroption.WithKeyPrefixer("GCP_"),
				configureroption.WithKeySuffixer("_VALUE"),
			},
			want: map[string]string{"GCP_USER_VALUE": "admin"},
		},
		{
			name:        "happy path multiple secrets",
			secretNames: []string{jsonSecretName, plainSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				jsonResource:  secretResponse([]byte(`{"JSON_KEY":"json"}`)),
				plainResource: secretResponse([]byte("plain")),
			},
			want: map[string]string{"JSON_KEY": "json", "plain-secret": "plain"},
		},
		{
			name:        "edge malformed JSON is plain text",
			secretNames: []string{plainSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				plainResource: secretResponse([]byte(`{"broken":`)),
			},
			want: map[string]string{"plain-secret": `{"broken":`},
		},
		{
			name:        "edge empty payload",
			secretNames: []string{plainSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				plainResource: secretResponse([]byte{}),
			},
			want: map[string]string{"plain-secret": ""},
		},
		{
			name:        "edge four encoded layers unwrap",
			secretNames: []string{jsonSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				jsonResource: secretResponse([]byte(`"\"{\\\"KEY\\\":\\\"value\\\"}\""`)),
			},
			want: map[string]string{"KEY": "value"},
		},
		{
			name:        "edge too many encoded layers remains plain text",
			secretNames: []string{plainSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				plainResource: secretResponse([]byte(wrapJSON(`{"KEY":"value"}`, 4))),
			},
			want: map[string]string{
				"plain-secret": wrapJSON(`{"KEY":"value"}`, 4),
			},
		},
		{
			name:        "happy path existing environment wins",
			secretNames: []string{jsonSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				jsonResource: secretResponse([]byte(`{"EXISTING":"loaded"}`)),
			},
			setupEnv: map[string]string{"EXISTING": "original"},
			want:     map[string]string{"EXISTING": "original"},
		},
		{
			name:        "happy path override existing environment",
			override:    true,
			secretNames: []string{jsonSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				jsonResource: secretResponse([]byte(`{"EXISTING":"loaded"}`)),
			},
			setupEnv: map[string]string{"EXISTING": "original"},
			want:     map[string]string{"EXISTING": "loaded"},
		},
		{
			name:        "edge raw value semantics",
			rawValue:    true,
			secretNames: []string{jsonSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				jsonResource: secretResponse([]byte(`{"RAW":"value"}`)),
			},
			want: map[string]string{"RAW": `"value"`},
		},
		{
			name:        "bad path missing secret",
			secretNames: []string{jsonSecretName},
			accessErrors: map[string]error{
				jsonResource: status.Error(codes.NotFound, "missing"),
			},
			wantErr: "get secret 'json-secret'",
		},
		{
			name:        "bad path API error",
			secretNames: []string{jsonSecretName},
			accessErrors: map[string]error{
				jsonResource: status.Error(codes.Internal, "unavailable"),
			},
			wantErr: "get secret 'json-secret'",
		},
		{
			name:        "bad path nil response",
			secretNames: []string{jsonSecretName},
			responses:   map[string]*secretmanagerpb.AccessSecretVersionResponse{},
			wantErr:     "secret payload for 'json-secret' required",
		},
		{
			name:        "bad path missing payload",
			secretNames: []string{jsonSecretName},
			responses: map[string]*secretmanagerpb.AccessSecretVersionResponse{
				jsonResource: {},
			},
			wantErr: "secret payload for 'json-secret' required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.setupEnv {
				t.Setenv(key, value)
			}
			for key := range tt.want {
				if _, exists := tt.setupEnv[key]; !exists {
					require.NoError(t, os.Unsetenv(key))
					t.Cleanup(func() {
						_ = os.Unsetenv(key)
					})
				}
			}

			client := &fakeSMClient{
				accessResponses: tt.responses,
				accessErrors:    tt.accessErrors,
			}
			gcpsm := newTestGCPSM(t, client, tt.override, tt.rawValue, tt.secretNames...)

			got, err := gcpsm.Load(context.Background(), tt.opts...)
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

func TestWrite(t *testing.T) {
	notFound := status.Error(codes.NotFound, "missing")
	internal := status.Error(codes.Internal, "failed")
	invalidWriteOption := configureroption.WithTarget("")

	tests := []struct {
		name           string
		values         map[string]interface{}
		secretNames    []string
		addErrors      []error
		createErr      error
		opts           []configureroption.WriteFunc
		wantErr        string
		wantAddCalls   int
		wantCreateCall bool
	}{
		{
			name:         "happy path add version to existing secret",
			values:       map[string]interface{}{"KEY": "value"},
			secretNames:  []string{"secret"},
			wantAddCalls: 1,
		},
		{
			name:           "happy path create missing secret then add version",
			values:         map[string]interface{}{"KEY": "value"},
			secretNames:    []string{"secret"},
			addErrors:      []error{notFound, nil},
			wantAddCalls:   2,
			wantCreateCall: true,
		},
		{
			name:           "edge concurrent creator already created secret",
			values:         map[string]interface{}{"KEY": "value"},
			secretNames:    []string{"secret"},
			addErrors:      []error{notFound, nil},
			createErr:      status.Error(codes.AlreadyExists, "created concurrently"),
			wantAddCalls:   2,
			wantCreateCall: true,
		},
		{
			name:        "bad path nil values",
			secretNames: []string{"secret"},
			wantErr:     "values required",
		},
		{
			name:        "bad path invalid write option",
			values:      map[string]interface{}{"KEY": "value"},
			secretNames: []string{"secret"},
			opts:        []configureroption.WriteFunc{invalidWriteOption},
			wantErr:     "target",
		},
		{
			name:        "edge missing secret names",
			values:      map[string]interface{}{"KEY": "value"},
			secretNames: []string{},
			wantErr:     "secret_names for write operation required",
		},
		{
			name:        "bad path marshal payload",
			values:      map[string]interface{}{"CHANNEL": make(chan int)},
			secretNames: []string{"secret"},
			wantErr:     "marshal secret data",
		},
		{
			name:         "bad path add version",
			values:       map[string]interface{}{"KEY": "value"},
			secretNames:  []string{"secret"},
			addErrors:    []error{internal},
			wantErr:      "add secret version",
			wantAddCalls: 1,
		},
		{
			name:           "bad path create missing secret",
			values:         map[string]interface{}{"KEY": "value"},
			secretNames:    []string{"secret"},
			addErrors:      []error{notFound},
			createErr:      internal,
			wantErr:        "create secret",
			wantAddCalls:   1,
			wantCreateCall: true,
		},
		{
			name:           "bad path add version after create",
			values:         map[string]interface{}{"KEY": "value"},
			secretNames:    []string{"secret"},
			addErrors:      []error{notFound, internal},
			wantErr:        "add secret version",
			wantAddCalls:   2,
			wantCreateCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeSMClient{
				addErrors: tt.addErrors,
				createErr: tt.createErr,
			}
			gcpsm := newTestGCPSM(t, client, false, false, tt.secretNames...)

			err := gcpsm.Write(context.Background(), tt.values, tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, client.addRequests, tt.wantAddCalls)
			if tt.wantAddCalls > 0 {
				assert.Equal(t, "projects/test-project/secrets/secret", client.addRequests[0].GetParent())
				assert.JSONEq(t, `{"KEY":"value"}`, string(client.addRequests[0].GetPayload().GetData()))
			}

			if tt.wantCreateCall {
				require.Len(t, client.createRequests, 1)
				request := client.createRequests[0]
				assert.Equal(t, "projects/test-project", request.GetParent())
				assert.Equal(t, "secret", request.GetSecretId())
				require.NotNil(t, request.GetSecret().GetReplication().GetAutomatic())
			} else {
				assert.Empty(t, client.createRequests)
			}
		})
	}
}

//////
// Helpers.
//////

func TestSecretKey(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		want       string
	}{
		{name: "happy path simple", secretName: "secret", want: "secret"},
		{name: "happy path path", secretName: "folder/nested/secret", want: "secret"},
		{name: "edge trailing slash", secretName: "folder/", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, secretKey(tt.secretName))
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "not found", err: status.Error(codes.NotFound, "missing"), want: true},
		{name: "wrapped not found", err: fmt.Errorf("wrapped: %w", status.Error(codes.NotFound, "missing")), want: true},
		{name: "internal", err: status.Error(codes.Internal, "failed")},
		{name: "plain error", err: errors.New("failed")},
		{name: "nil", err: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNotFound(tt.err))
		})
	}
}

func TestParseSecretData(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantObject bool
		want       map[string]interface{}
	}{
		{name: "object", value: `{"KEY":"value"}`, wantObject: true, want: map[string]interface{}{"KEY": "value"}},
		{name: "plain", value: "value"},
		{name: "JSON scalar", value: "42"},
		{name: "JSON null", value: "null", wantObject: true},
		{name: "JSON array", value: `["value"]`},
		{name: "four wrappers", value: `"\"{\\\"KEY\\\":\\\"value\\\"}\""`, wantObject: true, want: map[string]interface{}{"KEY": "value"}},
		{name: "too many wrappers", value: wrapJSON(`{"KEY":"value"}`, 4)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, isObject := parseSecretData(tt.value)
			assert.Equal(t, tt.wantObject, isObject)
			assert.Equal(t, tt.want, got)
		})
	}
}

func wrapJSON(value string, layers int) string {
	for range layers {
		encoded, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}

		value = string(encoded)
	}

	return value
}
