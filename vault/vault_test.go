// Package vault provides a Vault provider.
package vault

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	api "github.com/hashicorp/vault/api"
	"github.com/thalesfsp/configurer/option"
)

func TestNew(t *testing.T) {
	// Should only run if integration tests are enabled.
	//
	// NOTE: To run locally, comment the following.
	if os.Getenv("testing-integration") == "" {
		t.Skip("skipping integration test")
	}

	// NOTE: If running locally, you need to set this up.
	// t.Setenv("VAULT_ADDR", "http://localhost:8200")
	// t.Setenv("VAULT_TOKEN", "xyz")

	defaultAuth := &Auth{
		Address: os.Getenv("VAULT_ADDR"),
		Token:   os.Getenv("VAULT_TOKEN"),
	}

	appRoleAuth := &Auth{
		Address: defaultAuth.Address,
		AppRole: "test",
		Token:   defaultAuth.Token,
	}

	defaultSecretInformation := &SecretInformation{
		MountPath:  "secret",
		SecretPath: "mysql/webapp",
	}

	secretValue := map[string]interface{}{
		"user": "admin",
		"pass": "@dm|n",
	}

	wrongSecretValue := map[string]interface{}{
		"user": "admin",
		"pass": "123456",
	}

	defaultCleanUpFunc := func(ctx context.Context, client *api.Client, mountPath, secretPath string) {
		// Delete secret.
		_ = client.KVv2(mountPath).Delete(ctx, secretPath)

		// Delete policy.
		_ = client.Sys().DeletePolicy(appRoleAuth.AppRole)

		// Delete role.
		_, _ = client.Logical().Delete(fmt.Sprintf("auth/approle/role/%s", appRoleAuth.AppRole))
	}

	prefix := "TESTING_VAULT_"

	defaultOptions := []option.KeyFunc{
		option.WithKeyPrefixer(prefix),
		option.WithKeyCaser("upper"),
	}

	type args struct {
		override          bool
		authInformation   *Auth
		secretInformation *SecretInformation
	}
	tests := []struct {
		name          string
		args          args
		envFunc       func()
		opts          []option.KeyFunc
		credsAPI      *Auth
		secretAPI     *SecretInformation
		secretValue   map[string]interface{}
		expectedValue map[string]interface{}
		cleanUp       func(ctx context.Context, client *api.Client, mountPath, secretPath string)
		wantErr       bool
	}{
		{
			name: "should load - token",
			args: args{
				authInformation:   defaultAuth,
				secretInformation: defaultSecretInformation,
			},
			credsAPI:    defaultAuth,
			secretAPI:   defaultSecretInformation,
			envFunc:     func() {},
			opts:        defaultOptions,
			secretValue: secretValue,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     false,
		},
		{
			name: "should load - app role",
			args: args{
				authInformation:   appRoleAuth,
				secretInformation: defaultSecretInformation,
			},
			credsAPI:    appRoleAuth,
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
				authInformation:   defaultAuth,
				secretInformation: defaultSecretInformation,
			},
			credsAPI:  defaultAuth,
			secretAPI: defaultSecretInformation,
			envFunc: func() {
				t.Setenv("TESTING_VAULT_PASS", "123456")
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
				authInformation:   defaultAuth,
				secretInformation: defaultSecretInformation,
			},
			credsAPI:  defaultAuth,
			secretAPI: defaultSecretInformation,
			envFunc: func() {
				t.Setenv("TESTING_VAULT_PASS", "123456")
			},
			opts:        defaultOptions,
			secretValue: secretValue,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     false,
		},
		{
			name: "should load - missing auth information",
			args: args{
				authInformation:   nil,
				secretInformation: defaultSecretInformation,
			},
			credsAPI:  defaultAuth,
			secretAPI: defaultSecretInformation,
			envFunc: func() {
				os.Unsetenv("VAULT_TOKEN")
				os.Unsetenv("VAULT_ADDR")
			},
			opts:        defaultOptions,
			secretValue: secretValue,
			cleanUp:     defaultCleanUpFunc,
			wantErr:     true,
		},
		{
			name: "should load - missing secret information",
			args: args{
				authInformation:   defaultAuth,
				secretInformation: nil,
			},
			credsAPI:    defaultAuth,
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

		// Setup vault, and create a raw client.
		client, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			t.Fatalf("unable to initialize Vault client: %s", err)
		}

		if err := client.SetAddress(tt.credsAPI.Address); err != nil {
			t.Fatalf("unable to set Vault address: %s", err)
		}

		client.SetToken(tt.credsAPI.Token)

		//////
		// Ensure cleanup.
		//////

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			tt.cleanUp(
				ctx,
				client,
				tt.secretAPI.MountPath,
				tt.secretAPI.SecretPath,
			)
		}()

		// Enable AppRole.
		if err := client.Sys().EnableAuthWithOptions("approle", &api.EnableAuthOptions{
			Type: "approle",
		}); err != nil {
			if !strings.Contains(err.Error(), "already") {
				t.Fatalf("unable to enable AppRole: %s", err)
			}
		}

		// Create a policy.
		if err := client.Sys().PutPolicy(appRoleAuth.AppRole, `path "secret/data/mysql/webapp" {
			capabilities = ["read"]
		}`); err != nil {
			t.Fatalf("unable to create policy: %s", err)
		}

		// Create a new role.
		_, err = client.Logical().Write(
			fmt.Sprintf("auth/approle/role/%s", appRoleAuth.AppRole),
			map[string]interface{}{
				"token_policies": appRoleAuth.AppRole,
				"bind_secret_id": true,
				"secret_id_ttl":  "1m",
				"token_ttl":      "1m",
				"token_max_ttl":  "2m",
			})
		if err != nil {
			t.Fatalf("unable to create role: %s", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if _, err := client.
			KVv2(tt.secretAPI.MountPath).
			Put(
				ctx,
				tt.secretAPI.SecretPath,
				tt.secretValue,
			); err != nil {
			t.Fatalf("unable to create secret: %s", err)
		}

		// Should be enough time for the secret to be created.
		time.Sleep(1 * time.Second)

		//////
		// Run the test.
		//////

		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.override, tt.args.authInformation, tt.args.secretInformation)
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

					if os.Getenv(k) != v {
						t.Errorf("New() = %v, want %v", os.Getenv(k), v)
					}
				}
			}
		})
	}
}
