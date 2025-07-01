// Package awssm provides an AWS Secrets Manager provider.
package awssm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/thalesfsp/configurer/option"
)

func TestNew(t *testing.T) {
	// Should only run if integration tests are enabled.
	//
	// NOTE: To run locally, comment the following.
	if os.Getenv("testing-integration") == "" {
		t.Skip("skipping integration test")
	}

	t.Skip("skipping integration because it requires an AWS account. If you want to run this test, please provide your AWS credentials.")

	defaultConfig := &Config{
		// NOTE: Update bellow accordingly.
		//
		// Profile: os.Getenv("CONFIGURER_AWS_PROFILE"),
		// Region:    os.Getenv("AWS_REGION"),
		// AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		// SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}

	// Use a unique secret name to avoid conflicts.
	secretName := fmt.Sprintf("configurer-test-%d", time.Now().Unix())

	defaultSecretInformation := &SecretInformation{
		SecretNames: []string{secretName},
	}

	secretValue := map[string]interface{}{
		"user": "admin",
		"pass": "@dm|n",
	}

	wrongSecretValue := map[string]interface{}{
		"user": "admin",
		"pass": "123456",
	}

	defaultCleanUpFunc := func(ctx context.Context, client *secretsmanager.Client, secretName string) {
		// Delete secret.
		_, _ = client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	}

	prefix := "TESTING_AWSSM_"

	defaultOptions := []option.LoadKeyFunc{
		option.WithKeyPrefixer(prefix),
		option.WithKeyCaser("upper"),
	}

	type args struct {
		override          bool
		config            *Config
		secretInformation *SecretInformation
	}
	tests := []struct {
		name          string
		args          args
		envFunc       func()
		opts          []option.LoadKeyFunc
		configAPI     *Config
		secretAPI     *SecretInformation
		secretValue   map[string]interface{}
		expectedValue map[string]interface{}
		cleanUp       func(ctx context.Context, client *secretsmanager.Client, secretName string)
		wantErr       bool
	}{
		{
			name: "should load - valid configuration",
			args: args{
				config:            defaultConfig,
				secretInformation: defaultSecretInformation,
			},
			configAPI:   defaultConfig,
			secretAPI:   defaultSecretInformation,
			envFunc:     func() {},
			opts:        defaultOptions,
			secretValue: secretValue,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     false,
		},
		{
			name: "should load - should not override exported secret",
			args: args{
				override:          false,
				config:            defaultConfig,
				secretInformation: defaultSecretInformation,
			},
			configAPI: defaultConfig,
			secretAPI: defaultSecretInformation,
			envFunc: func() {
				t.Setenv("TESTING_AWSSM_PASS", "123456")
			},
			opts:          defaultOptions,
			secretValue:   secretValue,
			expectedValue: wrongSecretValue,
			cleanUp:       defaultCleanUpFunc,
			wantErr:       false,
		},
		{
			name: "should load - should override exported secret",
			args: args{
				override:          true,
				config:            defaultConfig,
				secretInformation: defaultSecretInformation,
			},
			configAPI: defaultConfig,
			secretAPI: defaultSecretInformation,
			envFunc: func() {
				t.Setenv("TESTING_AWSSM_PASS", "123456")
			},
			opts:        defaultOptions,
			secretValue: secretValue,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     false,
		},
		{
			name: "should load - missing config",
			args: args{
				config:            nil,
				secretInformation: defaultSecretInformation,
			},
			configAPI: defaultConfig,
			secretAPI: defaultSecretInformation,
			envFunc: func() {
				os.Unsetenv("AWS_ACCESS_KEY_ID")
				os.Unsetenv("AWS_SECRET_ACCESS_KEY")
				os.Unsetenv("AWS_REGION")
			},
			opts:        defaultOptions,
			secretValue: secretValue,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     true,
		},
		{
			name: "should load - missing secret information",
			args: args{
				config:            defaultConfig,
				secretInformation: nil,
			},
			configAPI:   defaultConfig,
			secretAPI:   defaultSecretInformation,
			envFunc:     func() {},
			opts:        defaultOptions,
			secretValue: secretValue,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		tt.envFunc()

		//////
		// Create a temp, new secret for testing purposes.
		//////

		// Setup AWS Secrets Manager client.
		provider, err := New(false, false, tt.configAPI, tt.secretAPI)
		if err != nil {
			// Check if it's an authentication error.
			if strings.Contains(err.Error(), "UnrecognizedClientException") || strings.Contains(err.Error(), "InvalidUserID.NotFound") || strings.Contains(err.Error(), "security token") {
				t.Skipf("AWS credentials not configured or invalid. Configure AWS credentials to run this integration test: %v", err)
			}
			t.Logf("Skipping test due to setup error: %v", err)
			continue
		}

		// Get the underlying AWS client for cleanup.
		awssm, ok := provider.(*AWSSM)
		if !ok {
			t.Fatalf("expected provider to be of type *AWSSM, got %T", provider)
		}

		client := awssm.client

		//////
		// Ensure cleanup.
		//////

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			tt.cleanUp(
				ctx,
				client,
				secretName,
			)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Create the secret for testing.
		if err := provider.Write(ctx, tt.secretValue); err != nil {
			t.Fatalf("unable to create secret: %s", err)
		}

		// Should be enough time for the secret to be created.
		time.Sleep(1 * time.Second)

		//////
		// Run the test.
		//////

		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.override, false, tt.args.config, tt.args.secretInformation)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != nil && err == nil {
				if _, err := got.Load(context.Background(), tt.opts...); (err != nil) != tt.wantErr {
					t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				}

				finalExpectedValue := tt.secretValue

				if tt.expectedValue != nil {
					finalExpectedValue = tt.expectedValue
				}

				// Iterate over secretValue and apply tt.opts to the keys.
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
