// Package awsssm provides an AWS Systems Manager Parameter Store provider.
package awsssm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/thalesfsp/configurer/option"
)

func TestNew(t *testing.T) {
	// Should only run if integration tests are enabled.
	//
	// NOTE: To run locally, set testing-integration=1.
	if os.Getenv("testing-integration") == "" {
		t.Skip("skipping integration test; set testing-integration=1 to run")
	}

	defaultConfig := &Config{
		// NOTE: Update bellow accordingly.
		//
		// Profile: os.Getenv("CONFIGURER_AWS_PROFILE"),
		// Region:    os.Getenv("AWS_REGION"),
		// AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		// SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}

	// Use a unique path to avoid conflicts.
	testPath := fmt.Sprintf("/configurer-test-%d", time.Now().Unix())

	defaultParamInfo := &ParameterInformation{
		Path:           testPath,
		Recursive:      true,
		WithDecryption: true,
	}

	paramValues := map[string]interface{}{
		"user": "admin",
		"pass": "@dm|n",
	}

	wrongParamValues := map[string]interface{}{
		"user": "admin",
		"pass": "123456",
	}

	defaultCleanUpFunc := func(ctx context.Context, client *ssm.Client, path string) {
		// Delete all parameters under the test path.
		for key := range paramValues {
			paramName := fmt.Sprintf("%s/%s", path, key)
			_, _ = client.DeleteParameter(ctx, &ssm.DeleteParameterInput{
				Name: aws.String(paramName),
			})
		}
	}

	prefix := "TESTING_AWSSSM_"

	defaultOptions := []option.LoadKeyFunc{
		option.WithKeyPrefixer(prefix),
		option.WithKeyCaser("upper"),
	}

	type args struct {
		override  bool
		config    *Config
		paramInfo *ParameterInformation
	}
	tests := []struct {
		name          string
		args          args
		envFunc       func()
		opts          []option.LoadKeyFunc
		configAPI     *Config
		paramInfoAPI  *ParameterInformation
		paramValues   map[string]interface{}
		expectedValue map[string]interface{}
		cleanUp       func(ctx context.Context, client *ssm.Client, path string)
		wantErr       bool
	}{
		{
			name: "should load - valid configuration",
			args: args{
				config:    defaultConfig,
				paramInfo: defaultParamInfo,
			},
			configAPI:    defaultConfig,
			paramInfoAPI: defaultParamInfo,
			envFunc:      func() {},
			opts:         defaultOptions,
			paramValues:  paramValues,
			cleanUp:      defaultCleanUpFunc,
			wantErr:      false,
		},
		{
			name: "should load - should not override exported parameter",
			args: args{
				override:  false,
				config:    defaultConfig,
				paramInfo: defaultParamInfo,
			},
			configAPI:    defaultConfig,
			paramInfoAPI: defaultParamInfo,
			envFunc: func() {
				t.Setenv("TESTING_AWSSSM_PASS", "123456")
			},
			opts:          defaultOptions,
			paramValues:   paramValues,
			expectedValue: wrongParamValues,
			cleanUp:       defaultCleanUpFunc,
			wantErr:       false,
		},
		{
			name: "should load - should override exported parameter",
			args: args{
				override:  true,
				config:    defaultConfig,
				paramInfo: defaultParamInfo,
			},
			configAPI:    defaultConfig,
			paramInfoAPI: defaultParamInfo,
			envFunc: func() {
				t.Setenv("TESTING_AWSSSM_PASS", "123456")
			},
			opts:        defaultOptions,
			paramValues: paramValues,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     false,
		},
		{
			name: "should load - missing config",
			args: args{
				config:    nil,
				paramInfo: defaultParamInfo,
			},
			configAPI:    defaultConfig,
			paramInfoAPI: defaultParamInfo,
			envFunc: func() {
				os.Unsetenv("AWS_ACCESS_KEY_ID")
				os.Unsetenv("AWS_SECRET_ACCESS_KEY")
				os.Unsetenv("AWS_REGION")
			},
			opts:        defaultOptions,
			paramValues: paramValues,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     true,
		},
		{
			name: "should load - missing parameter information",
			args: args{
				config:    defaultConfig,
				paramInfo: nil,
			},
			configAPI:    defaultConfig,
			paramInfoAPI: defaultParamInfo,
			envFunc:      func() {},
			opts:         defaultOptions,
			paramValues:  paramValues,
			cleanUp:      defaultCleanUpFunc,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		tt.envFunc()

		//////
		// Create temp parameters for testing purposes.
		//////

		// Setup AWS SSM client.
		provider, err := New(false, false, tt.configAPI, tt.paramInfoAPI)
		if err != nil {
			// Check if it's an authentication error.
			if strings.Contains(err.Error(), "UnrecognizedClientException") ||
				strings.Contains(err.Error(), "InvalidUserID.NotFound") ||
				strings.Contains(err.Error(), "security token") {
				t.Skipf("AWS credentials not configured or invalid. Configure AWS credentials to run this integration test: %v", err)
			}
			t.Logf("Skipping test due to setup error: %v", err)

			continue
		}

		// Get the underlying AWS client for cleanup.
		awsssm, ok := provider.(*AWSSSM)
		if !ok {
			t.Fatalf("expected provider to be of type *AWSSSM, got %T", provider)
		}

		client := awsssm.client

		//////
		// Ensure cleanup.
		//////

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			tt.cleanUp(
				ctx,
				client,
				testPath,
			)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Create the parameters for testing.
		if err := provider.Write(ctx, tt.paramValues); err != nil {
			t.Fatalf("unable to create parameters: %s", err)
		}

		// Should be enough time for the parameters to be created.
		time.Sleep(1 * time.Second)

		//////
		// Run the test.
		//////

		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.override, false, tt.args.config, tt.args.paramInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != nil && err == nil {
				if _, err := got.Load(context.Background(), tt.opts...); (err != nil) != tt.wantErr {
					t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				}

				finalExpectedValue := tt.paramValues

				if tt.expectedValue != nil {
					finalExpectedValue = tt.expectedValue
				}

				// Iterate over paramValues and apply tt.opts to the keys.
				for k, v := range finalExpectedValue {
					// Apply the options to the key.
					for _, opt := range tt.opts {
						k = opt(k)
					}

					if os.Getenv(k) != fmt.Sprintf("%v", v) {
						t.Errorf("New() = %v, want %v", os.Getenv(k), v)
					}
				}
			}
		})
	}
}

func TestExtractKeyFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/myapp/prod/DB_HOST", "DB_HOST"},
		{"/myapp/prod/nested/KEY", "KEY"},
		{"DB_HOST", "DB_HOST"},
		{"/single", "single"},
		{"", ""},
		{"/myapp/prod/", "prod"},           // trailing slash
		{"/myapp/prod/DB_HOST/", "DB_HOST"}, // trailing slash with value
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractKeyFromPath(tt.path)
			if got != tt.expected {
				t.Errorf("extractKeyFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestNewValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		paramInfo *ParameterInformation
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "missing config",
			config:    nil,
			paramInfo: &ParameterInformation{Path: "/test"},
			wantErr:   true,
			errMsg:    "config",
		},
		{
			name:      "missing param info",
			config:    &Config{Region: "us-east-1"},
			paramInfo: nil,
			wantErr:   true,
			errMsg:    "parameter information",
		},
		{
			name:   "missing both path and parameter names",
			config: &Config{Region: "us-east-1"},
			paramInfo: &ParameterInformation{
				Path:           "",
				ParameterNames: nil,
			},
			wantErr: true,
			errMsg:  "either path or parameter_names",
		},
		{
			name:   "valid with path",
			config: &Config{Region: "us-east-1"},
			paramInfo: &ParameterInformation{
				Path:           "/test",
				WithDecryption: true,
			},
			wantErr: false,
		},
		{
			name:   "valid with parameter names",
			config: &Config{Region: "us-east-1"},
			paramInfo: &ParameterInformation{
				ParameterNames: []string{"/test/param"},
				WithDecryption: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(false, false, tt.config, tt.paramInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("New() error = %v, should contain %q", err, tt.errMsg)
				}
			}
		})
	}
}
