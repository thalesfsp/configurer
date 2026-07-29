package awssm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/option"
)

//////
// Test helpers.
//////

const (
	getSecretValueTarget = "secretsmanager.GetSecretValue"
	createSecretTarget   = "secretsmanager.CreateSecret"
	updateSecretTarget   = "secretsmanager.UpdateSecret"
)

type fakeSecretsManagerResponse struct {
	status int
	body   string
}

type fakeSecretsManagerRequest struct {
	target string
	body   map[string]interface{}
}

type fakeSecretsManager struct {
	t         *testing.T
	server    *httptest.Server
	client    *http.Client
	mu        sync.Mutex
	responses map[string][]fakeSecretsManagerResponse
	requests  []fakeSecretsManagerRequest
}

func newFakeSecretsManager(
	t *testing.T,
	responses map[string][]fakeSecretsManagerResponse,
) *fakeSecretsManager {
	t.Helper()

	fake := &fakeSecretsManager{
		t:         t,
		responses: responses,
	}

	handler := http.HandlerFunc(fake.serveHTTP)
	fake.server = tryNewHTTPTestServer(handler)
	if fake.server != nil {
		fake.client = fake.server.Client()
	} else {
		listener := newMemoryListener()
		fake.server = &httptest.Server{
			Listener: listener,
			Config:   &http.Server{Handler: handler},
		}
		fake.server.Start()
		fake.client = &http.Client{
			Transport: &http.Transport{DialContext: listener.dialContext},
		}
	}

	t.Cleanup(func() {
		fake.client.CloseIdleConnections()
		fake.server.Close()
	})

	return fake
}

func tryNewHTTPTestServer(handler http.Handler) *httptest.Server {
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
	return "memory"
}

func (f *fakeSecretsManager) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		f.t.Errorf("request method = %q, want %q", r.Method, http.MethodPost)
	}

	if r.URL.Path != "/" {
		f.t.Errorf("request path = %q, want %q", r.URL.Path, "/")
	}

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		f.t.Errorf("read request body: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(requestBody, &body); err != nil {
		f.t.Errorf("decode request body %q: %v", requestBody, err)
	}

	target := r.Header.Get("X-Amz-Target")

	f.mu.Lock()
	f.requests = append(f.requests, fakeSecretsManagerRequest{
		target: target,
		body:   body,
	})

	targetResponses := f.responses[target]
	if len(targetResponses) == 0 {
		f.mu.Unlock()
		f.t.Errorf("unexpected AWS target %q", target)
		http.Error(w, "unexpected AWS target", http.StatusInternalServerError)

		return
	}

	response := targetResponses[0]
	f.responses[target] = targetResponses[1:]
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	if response.status >= http.StatusBadRequest {
		w.Header().Set("X-Amzn-Errortype", "ResourceNotFoundException")
	}

	w.WriteHeader(response.status)

	if _, err := io.WriteString(w, response.body); err != nil {
		f.t.Errorf("write response: %v", err)
	}
}

func (f *fakeSecretsManager) recordedRequests() []fakeSecretsManagerRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]fakeSecretsManagerRequest(nil), f.requests...)
}

func newTestAWSSM(
	t *testing.T,
	fake *fakeSecretsManager,
	override, rawValue bool,
	secretNames ...string,
) *AWSSM {
	t.Helper()

	awsConfig := aws.Config{
		Region: "us-east-1",
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		),
		BaseEndpoint: aws.String(fake.server.URL),
		HTTPClient:   fake.client,
	}

	got, err := NewWithConfig(
		override,
		rawValue,
		&Config{Region: "us-east-1"},
		&SecretInformation{SecretNames: secretNames},
		awsConfig,
	)
	require.NoError(t, err)

	provider, ok := got.(*AWSSM)
	require.True(t, ok)

	return provider
}

func awsSecretValueResponse(t *testing.T, secretString *string) string {
	t.Helper()

	body := map[string]interface{}{}
	if secretString != nil {
		body["SecretString"] = *secretString
	}

	encoded, err := json.Marshal(body)
	require.NoError(t, err)

	return string(encoded)
}

func secretString(value string) *string {
	return &value
}

func awsErrorResponse(message string) fakeSecretsManagerResponse {
	body, err := json.Marshal(map[string]string{
		"__type":  "ResourceNotFoundException",
		"message": message,
	})
	if err != nil {
		panic(err)
	}

	return fakeSecretsManagerResponse{
		status: http.StatusBadRequest,
		body:   string(body),
	}
}

func successfulResponse(body string) fakeSecretsManagerResponse {
	return fakeSecretsManagerResponse{
		status: http.StatusOK,
		body:   body,
	}
}

//////
// Constructor tests.
//////

func TestNewWithConfigUnit(t *testing.T) {
	validConfig := &Config{Region: "us-east-1"}
	validSecretInformation := &SecretInformation{SecretNames: []string{"unit/secret"}}
	awsConfig := aws.Config{
		Region: "us-east-1",
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		),
	}

	tests := []struct {
		name              string
		override          bool
		rawValue          bool
		config            *Config
		secretInformation *SecretInformation
		wantErrContains   string
	}{
		{
			name:              "happy path preserves provider options",
			override:          true,
			rawValue:          true,
			config:            validConfig,
			secretInformation: validSecretInformation,
		},
		{
			name:              "bad path rejects nil config",
			config:            nil,
			secretInformation: validSecretInformation,
			wantErrContains:   "config",
		},
		{
			name:              "bad path rejects nil secret information",
			config:            validConfig,
			secretInformation: nil,
			wantErrContains:   "secret information",
		},
		{
			name:              "edge case rejects empty secret names",
			config:            validConfig,
			secretInformation: &SecretInformation{},
			wantErrContains:   "SecretNames",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewWithConfig(
				tt.override,
				tt.rawValue,
				tt.config,
				tt.secretInformation,
				awsConfig,
			)

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErrContains)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)

			provider, ok := got.(*AWSSM)
			require.True(t, ok)
			assert.Equal(t, Name, provider.GetName())
			assert.Equal(t, tt.override, provider.GetOverride())
			assert.Equal(t, tt.rawValue, provider.GetRawValue())
			assert.Same(t, tt.config, provider.Config)
			assert.Same(t, tt.secretInformation, provider.SecretInformation)
			assert.NotNil(t, provider.client)
		})
	}
}

func TestNewUnit(t *testing.T) {
	type testCase struct {
		name              string
		config            *Config
		secretInformation *SecretInformation
		profileContents   string
		malformedProfile  bool
		wantErrContains   string
		wantAccessKey     string
		wantSecretKey     string
	}

	tests := []testCase{
		{
			name:              "bad path rejects nil config",
			secretInformation: &SecretInformation{SecretNames: []string{"unit/secret"}},
			wantErrContains:   "config",
		},
		{
			name:            "bad path rejects nil secret information",
			config:          &Config{Region: "us-east-1"},
			wantErrContains: "secret information",
		},
		{
			name:              "edge case rejects empty secret names",
			config:            &Config{Region: "us-east-1"},
			secretInformation: &SecretInformation{},
			wantErrContains:   "SecretNames",
		},
		{
			name:              "happy path loads default environment credentials",
			config:            &Config{Region: "us-east-1"},
			secretInformation: &SecretInformation{SecretNames: []string{"unit/secret"}},
			wantAccessKey:     "environment-key",
			wantSecretKey:     "environment-secret",
		},
		{
			name: "happy path overrides credentials from config",
			config: &Config{
				Region:    "us-east-1",
				AccessKey: "config-key",
				SecretKey: "config-secret",
			},
			secretInformation: &SecretInformation{SecretNames: []string{"unit/secret"}},
			wantAccessKey:     "config-key",
			wantSecretKey:     "config-secret",
		},
		{
			name: "happy path loads named profile",
			config: &Config{
				Region:  "us-east-1",
				Profile: "unit",
			},
			secretInformation: &SecretInformation{SecretNames: []string{"unit/secret"}},
			profileContents: "[profile unit]\nregion = us-east-1\n" +
				"aws_access_key_id = profile-key\n" +
				"aws_secret_access_key = profile-secret\n",
			wantAccessKey: "profile-key",
			wantSecretKey: "profile-secret",
		},
		{
			name: "bad path reports AWS config loading error",
			config: &Config{
				Region:  "us-east-1",
				Profile: "unit",
			},
			secretInformation: &SecretInformation{SecretNames: []string{"unit/secret"}},
			malformedProfile:  true,
			wantErrContains:   "load AWS config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWS_ACCESS_KEY_ID", "environment-key")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "environment-secret")
			t.Setenv("AWS_REGION", "us-east-1")
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:1")

			if tt.profileContents != "" || tt.malformedProfile {
				profilePath := filepath.Join(t.TempDir(), "config")
				contents := tt.profileContents
				if tt.malformedProfile {
					contents = "[profile unit\nregion = us-east-1\n"
				}

				require.NoError(t, os.WriteFile(profilePath, []byte(contents), 0o600))
				t.Setenv("AWS_CONFIG_FILE", profilePath)
			} else {
				t.Setenv("AWS_CONFIG_FILE", os.DevNull)
			}

			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", os.DevNull)

			got, err := New(true, true, tt.config, tt.secretInformation)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErrContains)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)

			provider, ok := got.(*AWSSM)
			require.True(t, ok)
			assert.Equal(t, Name, provider.GetName())
			assert.True(t, provider.GetOverride())
			assert.True(t, provider.GetRawValue())

			credentialsValue, err := provider.client.Options().Credentials.Retrieve(
				context.Background(),
			)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAccessKey, credentialsValue.AccessKeyID)
			assert.Equal(t, tt.wantSecretKey, credentialsValue.SecretAccessKey)
		})
	}
}

//////
// Load tests.
//////

func TestAWSSMLoadUnit(t *testing.T) {
	jsonSecret := `{"AWSSM_UNIT_BOOL":true,"AWSSM_UNIT_COUNT":3,"AWSSM_UNIT_JSON":"value"}`
	doubleEncodedSecret := doubleEncode(t, `{"AWSSM_UNIT_DOUBLE":"unwrapped"}`)
	invalidJSONKeySecret := "{\"BAD\\u0000KEY\":\"value\"}"

	tests := []struct {
		name            string
		secretNames     []string
		override        bool
		rawValue        bool
		responses       map[string][]fakeSecretsManagerResponse
		opts            []option.LoadKeyFunc
		existingEnv     map[string]string
		want            map[string]string
		wantErrContains string
		wantTargets     []string
	}{
		{
			name:        "happy path loads JSON secret values",
			secretNames: []string{"unit/json"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(awsSecretValueResponse(t, secretString(jsonSecret))),
				},
			},
			want: map[string]string{
				"AWSSM_UNIT_BOOL":  "true",
				"AWSSM_UNIT_COUNT": "3",
				"AWSSM_UNIT_JSON":  "value",
			},
			wantTargets: []string{getSecretValueTarget},
		},
		{
			name:        "happy path unwraps double encoded JSON secret",
			secretNames: []string{"unit/double"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(t, secretString(doubleEncodedSecret)),
					),
				},
			},
			want:        map[string]string{"AWSSM_UNIT_DOUBLE": "unwrapped"},
			wantTargets: []string{getSecretValueTarget},
		},
		{
			name:        "happy path loads plain secret with transformed leaf name",
			secretNames: []string{"unit/plain"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(t, secretString("plain-value")),
					),
				},
			},
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AWSSM_UNIT_"),
				option.WithKeyCaser("upper"),
			},
			want:        map[string]string{"AWSSM_UNIT_PLAIN": "plain-value"},
			wantTargets: []string{getSecretValueTarget},
		},
		{
			name:        "option handling preserves existing value without override",
			secretNames: []string{"unit/existing"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(t, secretString("new-value")),
					),
				},
			},
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AWSSM_UNIT_"),
				option.WithKeyCaser("upper"),
			},
			existingEnv: map[string]string{"AWSSM_UNIT_EXISTING": "old-value"},
			want:        map[string]string{"AWSSM_UNIT_EXISTING": "old-value"},
			wantTargets: []string{getSecretValueTarget},
		},
		{
			name:        "option handling overrides existing value",
			secretNames: []string{"unit/existing_override"},
			override:    true,
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(t, secretString("new-value")),
					),
				},
			},
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("AWSSM_UNIT_"),
				option.WithKeyCaser("upper"),
			},
			existingEnv: map[string]string{"AWSSM_UNIT_EXISTING_OVERRIDE": "old-value"},
			want:        map[string]string{"AWSSM_UNIT_EXISTING_OVERRIDE": "new-value"},
			wantTargets: []string{getSecretValueTarget},
		},
		{
			name:        "option handling exports raw JSON string value",
			secretNames: []string{"unit/raw"},
			rawValue:    true,
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(
							t,
							secretString(`{"AWSSM_UNIT_RAW":"value"}`),
						),
					),
				},
			},
			want:        map[string]string{"AWSSM_UNIT_RAW": `"value"`},
			wantTargets: []string{getSecretValueTarget},
		},
		{
			name:        "edge case accepts empty secret string",
			secretNames: []string{"AWSSM_UNIT_EMPTY"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(awsSecretValueResponse(t, secretString(""))),
				},
			},
			want:        map[string]string{"AWSSM_UNIT_EMPTY": ""},
			wantTargets: []string{getSecretValueTarget},
		},
		{
			name:        "bad path reports missing secret string",
			secretNames: []string{"unit/missing"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(awsSecretValueResponse(t, nil)),
				},
			},
			wantErrContains: "secret string",
			wantTargets:     []string{getSecretValueTarget},
		},
		{
			name:        "bad path wraps get secret API error",
			secretNames: []string{"unit/api-error"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {awsErrorResponse("get failed")},
			},
			wantErrContains: "get secret",
			wantTargets:     []string{getSecretValueTarget},
		},
		{
			name:        "bad path reports JSON object environment export error",
			secretNames: []string{"unit/invalid-json-key"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(t, secretString(invalidJSONKeySecret)),
					),
				},
			},
			wantErrContains: "export",
			wantTargets:     []string{getSecretValueTarget},
		},
		{
			name:        "bad path reports plain environment export error",
			secretNames: []string{"unit/\x00"},
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(t, secretString("plain-value")),
					),
				},
			},
			wantErrContains: "export",
			wantTargets:     []string{getSecretValueTarget},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.existingEnv {
				t.Setenv(key, value)
			}

			for key := range tt.want {
				if _, exists := tt.existingEnv[key]; !exists {
					t.Setenv(key, "")
				}
			}

			fake := newFakeSecretsManager(t, tt.responses)
			provider := newTestAWSSM(
				t,
				fake,
				tt.override,
				tt.rawValue,
				tt.secretNames...,
			)

			got, err := provider.Load(context.Background(), tt.opts...)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErrContains)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)

				for key, value := range tt.want {
					assert.Equal(t, value, os.Getenv(key))
				}
			}

			requests := fake.recordedRequests()
			require.Len(t, requests, len(tt.wantTargets))
			for i, target := range tt.wantTargets {
				assert.Equal(t, target, requests[i].target)
				assert.Equal(t, tt.secretNames[i], requests[i].body["SecretId"])
			}
		})
	}
}

//////
// Write tests.
//////

func TestAWSSMWriteUnit(t *testing.T) {
	writeValues := map[string]interface{}{
		"COUNT": 2,
		"KEY":   "value",
	}
	optionFailure := errors.New("write option failed")

	tests := []struct {
		name              string
		secretNames       []string
		values            map[string]interface{}
		responses         map[string][]fakeSecretsManagerResponse
		withOption        bool
		optionErr         error
		wantErrContains   string
		wantTargets       []string
		wantWrittenTarget string
	}{
		{
			name:            "bad path rejects nil values",
			secretNames:     []string{"unit/write"},
			wantErrContains: "values",
		},
		{
			name:            "bad path returns write option error",
			secretNames:     []string{"unit/write"},
			values:          writeValues,
			withOption:      true,
			optionErr:       optionFailure,
			wantErrContains: optionFailure.Error(),
		},
		{
			name:            "bad path rejects empty secret names",
			values:          writeValues,
			withOption:      true,
			wantErrContains: "secret_names",
		},
		{
			name:            "bad path reports JSON marshal error",
			secretNames:     []string{"unit/write"},
			values:          map[string]interface{}{"unsupported": make(chan int)},
			withOption:      true,
			wantErrContains: "marshal secret data",
		},
		{
			name:        "happy path creates missing secret using first name",
			secretNames: []string{"unit/write", "unit/ignored"},
			values:      writeValues,
			withOption:  true,
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {awsErrorResponse("missing")},
				createSecretTarget:   {successfulResponse(`{}`)},
			},
			wantTargets:       []string{getSecretValueTarget, createSecretTarget},
			wantWrittenTarget: createSecretTarget,
		},
		{
			name:        "happy path updates existing secret",
			secretNames: []string{"unit/write"},
			values:      writeValues,
			withOption:  true,
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(t, secretString("existing")),
					),
				},
				updateSecretTarget: {successfulResponse(`{}`)},
			},
			wantTargets:       []string{getSecretValueTarget, updateSecretTarget},
			wantWrittenTarget: updateSecretTarget,
		},
		{
			name:        "bad path wraps create secret API error",
			secretNames: []string{"unit/write"},
			values:      writeValues,
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {awsErrorResponse("missing")},
				createSecretTarget:   {awsErrorResponse("create failed")},
			},
			wantErrContains: "create secret",
			wantTargets:     []string{getSecretValueTarget, createSecretTarget},
		},
		{
			name:        "bad path wraps update secret API error",
			secretNames: []string{"unit/write"},
			values:      writeValues,
			responses: map[string][]fakeSecretsManagerResponse{
				getSecretValueTarget: {
					successfulResponse(
						awsSecretValueResponse(t, secretString("existing")),
					),
				},
				updateSecretTarget: {awsErrorResponse("update failed")},
			},
			wantErrContains: "update secret",
			wantTargets:     []string{getSecretValueTarget, updateSecretTarget},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeSecretsManager(t, tt.responses)
			secretNames := tt.secretNames
			if len(secretNames) == 0 {
				secretNames = []string{"temporary"}
			}

			provider := newTestAWSSM(t, fake, false, false, secretNames...)
			if len(tt.secretNames) == 0 {
				provider.SecretInformation.SecretNames = nil
			}

			optionCalled := false
			var opts []option.WriteFunc
			if tt.withOption {
				opts = append(opts, func(*option.Write) error {
					optionCalled = true

					return tt.optionErr
				})
			}

			err := provider.Write(context.Background(), tt.values, opts...)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErrContains)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.withOption && tt.values != nil, optionCalled)

			requests := fake.recordedRequests()
			require.Len(t, requests, len(tt.wantTargets))
			for i, target := range tt.wantTargets {
				assert.Equal(t, target, requests[i].target)
			}

			if tt.wantWrittenTarget != "" {
				writtenRequest := requests[len(requests)-1]
				if tt.wantWrittenTarget == updateSecretTarget {
					assert.Equal(t, "unit/write", writtenRequest.body["SecretId"])
					assert.NotContains(t, writtenRequest.body, "Name")
				} else {
					assert.Equal(t, "unit/write", writtenRequest.body["Name"])
					assert.NotContains(t, writtenRequest.body, "SecretId")
				}

				secretData, ok := writtenRequest.body["SecretString"].(string)
				require.True(t, ok)

				var decodedValues map[string]interface{}
				require.NoError(t, json.Unmarshal([]byte(secretData), &decodedValues))
				assert.Equal(t, "value", decodedValues["KEY"])
				assert.Equal(t, float64(2), decodedValues["COUNT"])
			}
		})
	}
}
