package awsssm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/option"
)

//////
// AWS SSM fake.
//////

type fakeSSMRequest struct {
	target string
	body   map[string]interface{}
}

type fakeSSMResponse struct {
	statusCode int
	body       interface{}
}

type fakeSSMRecorder struct {
	mu       sync.Mutex
	requests []fakeSSMRequest
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (r *fakeSSMRecorder) record(request fakeSSMRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests = append(r.requests, request)
}

func (r *fakeSSMRecorder) all() []fakeSSMRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]fakeSSMRequest(nil), r.requests...)
}

func newFakeSSMHTTPClient(
	t *testing.T,
	responder func(fakeSSMRequest) fakeSSMResponse,
) (*http.Client, *fakeSSMRecorder) {
	t.Helper()

	recorder := &fakeSSMRecorder{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("unexpected HTTP method: got %s, want %s", request.Method, http.MethodPost)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode SSM request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)

			return
		}

		fakeRequest := fakeSSMRequest{
			target: request.Header.Get("X-Amz-Target"),
			body:   body,
		}
		recorder.record(fakeRequest)

		response := responder(fakeRequest)
		if response.statusCode == 0 {
			response.statusCode = http.StatusOK
		}
		if response.body == nil {
			response.body = map[string]interface{}{}
		}

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(response.statusCode)
		if err := json.NewEncoder(w).Encode(response.body); err != nil {
			t.Errorf("encode SSM response: %v", err)
		}
	})
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			return response.Result(), nil
		}),
	}

	return client, recorder
}

func configureFakeAWS(t *testing.T, endpoint string) {
	t.Helper()

	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ENDPOINT_URL", endpoint)
}

func newFakeAWSSSM(
	t *testing.T,
	httpClient *http.Client,
	override bool,
	rawValue bool,
	paramInfo *ParameterInformation,
) *AWSSSM {
	t.Helper()

	configureFakeAWS(t, "http://ssm.test")
	awsConfig, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion("us-east-1"),
	)
	require.NoError(t, err)
	awsConfig.HTTPClient = httpClient

	result, err := NewWithConfig(
		override,
		rawValue,
		&Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		},
		paramInfo,
		awsConfig,
	)
	require.NoError(t, err)

	typed, ok := result.(*AWSSSM)
	require.True(t, ok)

	return typed
}

func stringSlice(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		stringItem, ok := item.(string)
		if !ok {
			return nil
		}

		result = append(result, stringItem)
	}

	return result
}

//////
// Constructors.
//////

func TestNewWithConfigUnit(t *testing.T) {
	tests := []struct {
		name      string
		override  bool
		rawValue  bool
		config    *Config
		paramInfo *ParameterInformation
		wantErr   string
	}{
		{
			name:      "missing config",
			paramInfo: &ParameterInformation{Path: "/app"},
			wantErr:   "config",
		},
		{
			name:    "missing parameter information",
			config:  &Config{Region: "us-east-1"},
			wantErr: "parameter information",
		},
		{
			name:      "missing path and names",
			config:    &Config{Region: "us-east-1"},
			paramInfo: &ParameterInformation{},
			wantErr:   "either path or parameter_names",
		},
		{
			name:     "valid path preserves provider options",
			override: true,
			rawValue: true,
			config:   &Config{Region: "us-east-1"},
			paramInfo: &ParameterInformation{
				Path: "/app",
			},
		},
		{
			name:   "valid parameter names",
			config: &Config{Region: "us-east-1"},
			paramInfo: &ParameterInformation{
				ParameterNames: []string{"/app/key"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewWithConfig(
				tt.override,
				tt.rawValue,
				tt.config,
				tt.paramInfo,
				aws.Config{Region: "us-east-1"},
			)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)

				return
			}

			require.NoError(t, err)
			require.IsType(t, &AWSSSM{}, result)
			assert.Equal(t, Name, result.GetName())
			assert.Equal(t, tt.override, result.GetOverride())
			assert.Equal(t, tt.rawValue, result.GetRawValue())
		})
	}
}

func TestNewAWSConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		sharedConfig   string
		wantErr        string
		wantCredential string
	}{
		{
			name: "explicit access keys",
			config: &Config{
				Region:    "us-east-1",
				AccessKey: "config-access-key",
				SecretKey: "config-secret-key",
			},
			wantCredential: "config-access-key",
		},
		{
			name: "named profile",
			config: &Config{
				Region:    "us-east-1",
				Profile:   "unit",
				AccessKey: "profile-override-key",
				SecretKey: "profile-override-secret",
			},
			sharedConfig:   "[profile unit]\nregion = us-east-1\n",
			wantCredential: "profile-override-key",
		},
		{
			name: "malformed named profile",
			config: &Config{
				Region:  "us-east-1",
				Profile: "broken",
			},
			sharedConfig: "[profile broken\n",
			wantErr:      "load AWS config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configureFakeAWS(t, "http://ssm.test")

			if tt.sharedConfig != "" {
				configPath := filepath.Join(t.TempDir(), "config")
				require.NoError(t, os.WriteFile(configPath, []byte(tt.sharedConfig), 0o600))
				t.Setenv("AWS_CONFIG_FILE", configPath)
			}

			result, err := New(false, false, tt.config, &ParameterInformation{Path: "/app"})
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)

				return
			}

			require.NoError(t, err)

			awsssmProvider, ok := result.(*AWSSSM)
			require.True(t, ok)

			credentials, err := awsssmProvider.client.Options().Credentials.Retrieve(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.wantCredential, credentials.AccessKeyID)
		})
	}
}

//////
// Load.
//////

func TestAWSSSMLoadByPath(t *testing.T) {
	tests := []struct {
		name            string
		override        bool
		rawValue        bool
		seedEnvironment map[string]string
		opts            []option.LoadKeyFunc
		responder       func(fakeSSMRequest) fakeSSMResponse
		want            map[string]string
		wantErr         string
		wantRequests    int
	}{
		{
			name: "loads paginated parameters and applies key options",
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("TEST_AWSSSM_"),
				option.WithKeyCaser("upper"),
			},
			responder: func(request fakeSSMRequest) fakeSSMResponse {
				if request.body["NextToken"] == "next-page" {
					return fakeSSMResponse{
						body: map[string]interface{}{
							"Parameters": []map[string]interface{}{
								{"Name": "/app/port", "Value": "8080", "Type": "String"},
							},
						},
					}
				}

				return fakeSSMResponse{
					body: map[string]interface{}{
						"NextToken": "next-page",
						"Parameters": []map[string]interface{}{
							{"Name": "/app/db_host", "Value": "database", "Type": "String"},
							{"Name": "/app/servers", "Value": "one,two", "Type": "StringList"},
							{"Value": "missing-name", "Type": "String"},
							{"Name": "/app/missing-value", "Type": "String"},
						},
					},
				}
			},
			want: map[string]string{
				"TEST_AWSSSM_DB_HOST": "database",
				"TEST_AWSSSM_PORT":    "8080",
				"TEST_AWSSSM_SERVERS": "one,two",
			},
			wantRequests: 2,
		},
		{
			name:     "keeps an existing environment value without override",
			override: false,
			seedEnvironment: map[string]string{
				"KEEP": "existing",
			},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					body: map[string]interface{}{
						"Parameters": []map[string]interface{}{
							{"Name": "/app/KEEP", "Value": "replacement", "Type": "String"},
						},
					},
				}
			},
			want: map[string]string{
				"KEEP": "existing",
			},
			wantRequests: 1,
		},
		{
			name:     "overrides an existing environment value",
			override: true,
			seedEnvironment: map[string]string{
				"KEEP": "existing",
			},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					body: map[string]interface{}{
						"Parameters": []map[string]interface{}{
							{"Name": "/app/KEEP", "Value": "replacement", "Type": "String"},
						},
					},
				}
			},
			want: map[string]string{
				"KEEP": "replacement",
			},
			wantRequests: 1,
		},
		{
			name:     "formats a raw value",
			rawValue: true,
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					body: map[string]interface{}{
						"Parameters": []map[string]interface{}{
							{"Name": "/app/RAW", "Value": "hello", "Type": "String"},
						},
					},
				}
			},
			want: map[string]string{
				"RAW": `"hello"`,
			},
			wantRequests: 1,
		},
		{
			name: "returns an empty map for an empty parameter list",
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					body: map[string]interface{}{
						"Parameters": []map[string]interface{}{},
					},
				}
			},
			want:         map[string]string{},
			wantRequests: 1,
		},
		{
			name: "wraps a get parameters by path error",
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					statusCode: http.StatusBadRequest,
					body: map[string]interface{}{
						"__type":  "ValidationException",
						"message": "path failed",
					},
				}
			},
			wantErr:      "get parameters by path '/app'",
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient, recorder := newFakeSSMHTTPClient(t, tt.responder)
			for key, value := range tt.want {
				t.Setenv(key, "")
				if seededValue, ok := tt.seedEnvironment[key]; ok {
					t.Setenv(key, seededValue)
				}
				_ = value
			}

			awsssmProvider := newFakeAWSSSM(
				t,
				httpClient,
				tt.override,
				tt.rawValue,
				&ParameterInformation{
					Path:           "/app",
					Recursive:      true,
					WithDecryption: true,
				},
			)

			result, err := awsssmProvider.Load(context.Background(), tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
				for key, value := range tt.want {
					assert.Equal(t, value, os.Getenv(key))
				}
			}

			requests := recorder.all()
			require.Len(t, requests, tt.wantRequests)
			for _, request := range requests {
				assert.Equal(t, "AmazonSSM.GetParametersByPath", request.target)
				assert.Equal(t, "/app", request.body["Path"])
				assert.Equal(t, true, request.body["Recursive"])
				assert.Equal(t, true, request.body["WithDecryption"])
			}
			if tt.wantRequests == 2 {
				assert.NotContains(t, requests[0].body, "NextToken")
				assert.Equal(t, "next-page", requests[1].body["NextToken"])
			}
		})
	}
}

func TestAWSSSMLoadByNames(t *testing.T) {
	names := []string{
		"/app/name_01",
		"/app/name_02",
		"/app/name_03",
		"/app/name_04",
		"/app/name_05",
		"/app/name_06",
		"/app/name_07",
		"/app/name_08",
		"/app/name_09",
		"/app/name_10",
		"/app/name_11",
	}
	wantBatchedValues := make(map[string]string, len(names))
	for _, name := range names {
		key := "TEST_" + extractKeyFromPath(name)
		wantBatchedValues[key] = "value-" + extractKeyFromPath(name)
	}

	tests := []struct {
		name         string
		names        []string
		opts         []option.LoadKeyFunc
		responder    func(fakeSSMRequest) fakeSSMResponse
		want         map[string]string
		wantErr      string
		wantRequests int
	}{
		{
			name:  "loads names in batches and applies key options",
			names: names,
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("TEST_"),
			},
			responder: func(request fakeSSMRequest) fakeSSMResponse {
				parameters := make([]map[string]interface{}, 0)
				for _, name := range stringSlice(request.body["Names"]) {
					parameters = append(parameters, map[string]interface{}{
						"Name":  name,
						"Value": "value-" + extractKeyFromPath(name),
						"Type":  "SecureString",
					})
				}

				return fakeSSMResponse{
					body: map[string]interface{}{
						"Parameters": parameters,
					},
				}
			},
			want:         wantBatchedValues,
			wantRequests: 2,
		},
		{
			name:  "returns an empty map for an empty response",
			names: []string{"/app/missing"},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					body: map[string]interface{}{
						"Parameters": []map[string]interface{}{},
					},
				}
			},
			want:         map[string]string{},
			wantRequests: 1,
		},
		{
			name:  "reports invalid parameters",
			names: []string{"/app/missing", "/app/also-missing"},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					body: map[string]interface{}{
						"InvalidParameters": []string{"/app/missing", "/app/also-missing"},
					},
				}
			},
			wantErr:      "parameters: /app/missing, /app/also-missing",
			wantRequests: 1,
		},
		{
			name:  "wraps a get parameters error",
			names: []string{"/app/failure"},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					statusCode: http.StatusBadRequest,
					body: map[string]interface{}{
						"__type":  "ValidationException",
						"message": "names failed",
					},
				}
			},
			wantErr:      "get parameters",
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient, recorder := newFakeSSMHTTPClient(t, tt.responder)
			for key := range tt.want {
				t.Setenv(key, "")
			}

			awsssmProvider := newFakeAWSSSM(
				t,
				httpClient,
				true,
				false,
				&ParameterInformation{
					ParameterNames: tt.names,
					WithDecryption: true,
				},
			)

			result, err := awsssmProvider.Load(context.Background(), tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
				for key, value := range tt.want {
					assert.Equal(t, value, os.Getenv(key))
				}
			}

			requests := recorder.all()
			require.Len(t, requests, tt.wantRequests)
			for _, request := range requests {
				assert.Equal(t, "AmazonSSM.GetParameters", request.target)
				assert.Equal(t, true, request.body["WithDecryption"])
				assert.NotEmpty(t, stringSlice(request.body["Names"]))
			}
			if tt.wantRequests == 2 {
				assert.Len(t, stringSlice(requests[0].body["Names"]), 10)
				assert.Len(t, stringSlice(requests[1].body["Names"]), 1)
			}
		})
	}
}

func TestAWSSSMLoadCombinedSources(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "loads both a path and explicit names"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient, recorder := newFakeSSMHTTPClient(t, func(request fakeSSMRequest) fakeSSMResponse {
				switch request.target {
				case "AmazonSSM.GetParametersByPath":
					return fakeSSMResponse{
						body: map[string]interface{}{
							"Parameters": []map[string]interface{}{
								{"Name": "/app/path-key", "Value": "path-value", "Type": "String"},
							},
						},
					}
				case "AmazonSSM.GetParameters":
					return fakeSSMResponse{
						body: map[string]interface{}{
							"Parameters": []map[string]interface{}{
								{"Name": "/other/name-key", "Value": "name-value", "Type": "String"},
							},
						},
					}
				default:
					return fakeSSMResponse{statusCode: http.StatusBadRequest}
				}
			})
			t.Setenv("path-key", "")
			t.Setenv("name-key", "")

			awsssmProvider := newFakeAWSSSM(
				t,
				httpClient,
				true,
				false,
				&ParameterInformation{
					Path:           "/app",
					ParameterNames: []string{"/other/name-key"},
				},
			)

			result, err := awsssmProvider.Load(context.Background())
			require.NoError(t, err)
			assert.Equal(t, map[string]string{
				"path-key": "path-value",
				"name-key": "name-value",
			}, result)
			assert.Len(t, recorder.all(), 2)
		})
	}
}

//////
// Write.
//////

func TestAWSSSMWrite(t *testing.T) {
	tests := []struct {
		name          string
		paramInfo     *ParameterInformation
		values        map[string]interface{}
		opts          []option.WriteFunc
		responder     func(fakeSSMRequest) fakeSSMResponse
		want          map[string]string
		wantErr       string
		wantRequests  int
		wantTarget    string
		wantOverwrite bool
	}{
		{
			name: "writes supported value types under a normalized path",
			paramInfo: &ParameterInformation{
				Path: "app/",
			},
			values: map[string]interface{}{
				"string": "value",
				"list":   []string{"one", "two"},
				"number": 42,
			},
			opts: []option.WriteFunc{
				option.WithEnvironment("testing"),
				option.WithVariable(true),
			},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{}
			},
			want: map[string]string{
				"/app/string": "value",
				"/app/list":   "one,two",
				"/app/number": "42",
			},
			wantRequests:  3,
			wantTarget:    "AmazonSSM.PutParameter",
			wantOverwrite: true,
		},
		{
			name: "derives the base path from the first parameter name",
			paramInfo: &ParameterInformation{
				ParameterNames: []string{"/named/source"},
			},
			values: map[string]interface{}{
				"key": "value",
			},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{}
			},
			want: map[string]string{
				"/named/key": "value",
			},
			wantRequests:  1,
			wantTarget:    "AmazonSSM.PutParameter",
			wantOverwrite: true,
		},
		{
			name: "falls back to the root path",
			paramInfo: &ParameterInformation{
				ParameterNames: []string{"source"},
			},
			values: map[string]interface{}{
				"key": "value",
			},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{}
			},
			want: map[string]string{
				"/key": "value",
			},
			wantRequests:  1,
			wantTarget:    "AmazonSSM.PutParameter",
			wantOverwrite: true,
		},
		{
			name: "accepts an empty values map",
			paramInfo: &ParameterInformation{
				Path: "/app",
			},
			values: map[string]interface{}{},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{}
			},
			want:         map[string]string{},
			wantRequests: 0,
		},
		{
			name: "rejects nil values",
			paramInfo: &ParameterInformation{
				Path: "/app",
			},
			values:       nil,
			wantErr:      "values",
			wantRequests: 0,
		},
		{
			name: "returns an invalid write option error before calling SSM",
			paramInfo: &ParameterInformation{
				Path: "/app",
			},
			values: map[string]interface{}{
				"key": "value",
			},
			opts: []option.WriteFunc{
				option.WithTarget(""),
			},
			wantErr:      "target",
			wantRequests: 0,
		},
		{
			name: "wraps a put parameter error",
			paramInfo: &ParameterInformation{
				Path: "/app",
			},
			values: map[string]interface{}{
				"failure": "value",
			},
			responder: func(fakeSSMRequest) fakeSSMResponse {
				return fakeSSMResponse{
					statusCode: http.StatusBadRequest,
					body: map[string]interface{}{
						"__type":  "ValidationException",
						"message": "put failed",
					},
				}
			},
			want: map[string]string{
				"/app/failure": "value",
			},
			wantErr:       "put parameter '/app/failure'",
			wantRequests:  1,
			wantTarget:    "AmazonSSM.PutParameter",
			wantOverwrite: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responder := tt.responder
			if responder == nil {
				responder = func(fakeSSMRequest) fakeSSMResponse {
					return fakeSSMResponse{}
				}
			}

			httpClient, recorder := newFakeSSMHTTPClient(t, responder)
			awsssmProvider := newFakeAWSSSM(t, httpClient, false, false, tt.paramInfo)

			err := awsssmProvider.Write(context.Background(), tt.values, tt.opts...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			requests := recorder.all()
			require.Len(t, requests, tt.wantRequests)
			for _, request := range requests {
				assert.Equal(t, tt.wantTarget, request.target)
				name, ok := request.body["Name"].(string)
				require.True(t, ok)
				assert.Equal(t, tt.want[name], request.body["Value"])
				assert.Equal(t, "SecureString", request.body["Type"])
				assert.Equal(t, tt.wantOverwrite, request.body["Overwrite"])
			}
		})
	}
}
