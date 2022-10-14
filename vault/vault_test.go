// Package vault provides a Vault provider.
package vault

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	api "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/vault"
	"github.com/thalesfsp/configurer/option"
)

func createTestVault(t *testing.T) (net.Listener, *api.Client) {
	t.Helper()

	// Create an in-memory, unsealed core (the "backend", if you will).
	core, keyShares, rootToken := vault.TestCoreUnsealed(t)
	_ = keyShares

	core.SetLogLevel(hclog.NoLevel)
	core.Logger().SetLevel(hclog.NoLevel)

	// Start an HTTP server for the core.
	ln, addr := http.TestServer(t, core)

	// Create a client that talks to the server, initially authenticating with
	// the root token.
	conf := api.DefaultConfig()
	conf.Address = addr

	client, err := api.NewClient(conf)
	if err != nil {
		t.Fatal(err)
	}
	client.SetToken(rootToken)

	// Setup required secrets, policies, etc.
	_, err = client.Logical().Write("secret/foo", map[string]interface{}{
		"secret": "bar",
	})
	if err != nil {
		t.Fatal(err)
	}

	return ln, client
}

func TestNew(t *testing.T) {
	prefix := "TESTING_VAULT_"

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
				secretInformation: &SecretInformation{
					MountPath:  "secret",
					SecretPath: "test",
				},
			},
			credsAPI: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "root",
			},
			secretAPI: &SecretInformation{
				MountPath:  "secret",
				SecretPath: "mysql/webapp",
			},
			envFunc: func() {
				os.Unsetenv("VAULT_ADDR")
				os.Unsetenv("VAULT_TOKEN")

				// Comment if using namespace.
				os.Unsetenv("VAULT_NAMESPACE")

				// TODO: Uncomment bellow if you want to run the test locally.
				t.Setenv("VAULT_ADDR", "http://127.0.0.1:8200")
				t.Setenv("VAULT_TOKEN", "root")

				// Comment if using namespace.
				t.Setenv("VAULT_NAMESPACE", "")
			},
			opts: []option.KeyFunc{
				option.WithKeyPrefixer(prefix),
				option.WithKeyCaser(option.Uppercase),
			},
			secretValue: map[string]interface{}{
				"user": "admin",
				"pass": "@dm|n",
			},
			cleanUp: func(ctx context.Context, client *api.Client, mountPath, secretPath string) {
				_ = client.KVv2(mountPath).Delete(ctx, secretPath)
			},
			wantErr: false,
		},
		{
			name: "should load - app role",
			args: args{
				secretInformation: &SecretInformation{
					MountPath:  "secret",
					SecretPath: "mysql/webapp",
				},
			},
			credsAPI: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "root",
			},
			secretAPI: &SecretInformation{
				MountPath:  "secret",
				SecretPath: "mysql/webapp",
			},
			envFunc: func() {
				os.Unsetenv("VAULT_ADDR")
				os.Unsetenv("VAULT_APP_ROLE")
				os.Unsetenv("VAULT_TOKEN")

				// Comment if using namespace.
				os.Unsetenv("VAULT_NAMESPACE")

				// TODO: Uncomment bellow if you want to run the test locally.
				t.Setenv("VAULT_ADDR", "http://127.0.0.1:8200")
				t.Setenv("VAULT_APP_ROLE", "jenkins")
				t.Setenv("VAULT_TOKEN", "root")

				// Comment if using namespace.
				t.Setenv("VAULT_NAMESPACE", "")
			},
			opts: []option.KeyFunc{
				option.WithKeyPrefixer(prefix),
				option.WithKeyCaser(option.Uppercase),
			},
			secretValue: map[string]interface{}{
				"user": "admin",
				"pass": "@dm|n",
			},
			cleanUp: func(ctx context.Context, client *api.Client, mountPath, secretPath string) {
				_ = client.KVv2(mountPath).Delete(ctx, secretPath)
			},
			wantErr: false,
		},
		{
			name: "should load - pass auth information",
			args: args{
				authInformation: &Auth{
					Address:   "http://127.0.0.1:8200",
					AppRole:   "jenkins",
					Namespace: "",
					Token:     "root",
				},
				secretInformation: &SecretInformation{
					MountPath:  "secret",
					SecretPath: "mysql/webapp",
				},
			},
			credsAPI: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "root",
			},
			secretAPI: &SecretInformation{
				MountPath:  "secret",
				SecretPath: "mysql/webapp",
			},
			envFunc: func() {
				os.Unsetenv("VAULT_ADDR")
				os.Unsetenv("VAULT_APP_ROLE")
				os.Unsetenv("VAULT_TOKEN")

				// Comment if using namespace.
				os.Unsetenv("VAULT_NAMESPACE")

				t.Setenv("VAULT_ADDR", "http://127.0.0.1:8200")
				t.Setenv("VAULT_APP_ROLE", "jenkins")
				t.Setenv("VAULT_TOKEN", "root")
			},
			opts: []option.KeyFunc{
				option.WithKeyPrefixer(prefix),
				option.WithKeyCaser(option.Uppercase),
			},
			secretValue: map[string]interface{}{
				"user": "admin",
				"pass": "@dm|n",
			},
			cleanUp: func(ctx context.Context, client *api.Client, mountPath, secretPath string) {
				_ = client.KVv2(mountPath).Delete(ctx, secretPath)
			},
			wantErr: false,
		},
		{
			name: "should load - should not override exported secret",
			args: args{
				override: false,
				authInformation: &Auth{
					Address:   "http://127.0.0.1:8200",
					AppRole:   "jenkins",
					Namespace: "",
					Token:     "root",
				},
				secretInformation: &SecretInformation{
					MountPath:  "secret",
					SecretPath: "mysql/webapp",
				},
			},
			credsAPI: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "root",
			},
			secretAPI: &SecretInformation{
				MountPath:  "secret",
				SecretPath: "mysql/webapp",
			},
			envFunc: func() {
				os.Unsetenv("VAULT_ADDR")
				os.Unsetenv("VAULT_APP_ROLE")
				os.Unsetenv("VAULT_TOKEN")

				// Comment if using namespace.
				os.Unsetenv("VAULT_NAMESPACE")

				t.Setenv("VAULT_ADDR", "http://127.0.0.1:8200")
				t.Setenv("VAULT_APP_ROLE", "jenkins")
				t.Setenv("VAULT_TOKEN", "root")

				t.Setenv("TESTING_VAULT_PASS", "123456")
			},
			opts: []option.KeyFunc{
				option.WithKeyPrefixer(prefix),
				option.WithKeyCaser(option.Uppercase),
			},
			secretValue: map[string]interface{}{
				"user": "admin",
				"pass": "@dm|n",
			},
			expectedValue: map[string]interface{}{
				"user": "admin",
				"pass": "123456",
			},
			cleanUp: func(ctx context.Context, client *api.Client, mountPath, secretPath string) {
				_ = client.KVv2(mountPath).Delete(ctx, secretPath)
			},
			wantErr: false,
		},
		{
			name: "should load - should override exported secret",
			args: args{
				override: true,
				authInformation: &Auth{
					Address:   "http://127.0.0.1:8200",
					AppRole:   "jenkins",
					Namespace: "",
					Token:     "root",
				},
				secretInformation: &SecretInformation{
					MountPath:  "secret",
					SecretPath: "mysql/webapp",
				},
			},
			credsAPI: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "root",
			},
			secretAPI: &SecretInformation{
				MountPath:  "secret",
				SecretPath: "mysql/webapp",
			},
			envFunc: func() {
				os.Unsetenv("VAULT_ADDR")
				os.Unsetenv("VAULT_APP_ROLE")
				os.Unsetenv("VAULT_TOKEN")

				// Comment if using namespace.
				os.Unsetenv("VAULT_NAMESPACE")

				t.Setenv("VAULT_ADDR", "http://127.0.0.1:8200")
				t.Setenv("VAULT_APP_ROLE", "jenkins")
				t.Setenv("VAULT_TOKEN", "root")

				t.Setenv("TESTING_VAULT_PASS", "123456")
			},
			opts: []option.KeyFunc{
				option.WithKeyPrefixer(prefix),
				option.WithKeyCaser(option.Uppercase),
			},
			secretValue: map[string]interface{}{
				"user": "admin",
				"pass": "@dm|n",
			},
			expectedValue: map[string]interface{}{
				"user": "admin",
				"pass": "@dm|n",
			},
			cleanUp: func(ctx context.Context, client *api.Client, mountPath, secretPath string) {
				_ = client.KVv2(mountPath).Delete(ctx, secretPath)
			},
			wantErr: false,
		},
		{
			name: "should load - missing auth information",
			args: args{
				authInformation: nil,
				secretInformation: &SecretInformation{
					MountPath:  "secret",
					SecretPath: "mysql/webapp",
				},
			},
			credsAPI: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "root",
			},
			secretAPI: &SecretInformation{
				MountPath:  "secret",
				SecretPath: "mysql/webapp",
			},
			envFunc: func() {
				os.Unsetenv("VAULT_ADDR")
				os.Unsetenv("VAULT_APP_ROLE")
				os.Unsetenv("VAULT_TOKEN")

				// Comment if using namespace.
				os.Unsetenv("VAULT_NAMESPACE")
			},
			opts: []option.KeyFunc{
				option.WithKeyPrefixer(prefix),
				option.WithKeyCaser(option.Uppercase),
			},
			secretValue: map[string]interface{}{
				"user": "admin",
				"pass": "@dm|n",
			},
			cleanUp: func(ctx context.Context, client *api.Client, mountPath, secretPath string) {
				_ = client.KVv2(mountPath).Delete(ctx, secretPath)
			},
			wantErr: true,
		},
		{
			name: "should load - missing secret information",
			args: args{
				authInformation:   nil,
				secretInformation: &SecretInformation{},
			},
			credsAPI: &Auth{
				Address: "http://127.0.0.1:8200",
				Token:   "root",
			},
			secretAPI: &SecretInformation{
				MountPath:  "secret",
				SecretPath: "mysql/webapp",
			},
			envFunc: func() {
				os.Unsetenv("VAULT_ADDR")
				os.Unsetenv("VAULT_APP_ROLE")
				os.Unsetenv("VAULT_TOKEN")

				// Comment if using namespace.
				os.Unsetenv("VAULT_NAMESPACE")

				t.Setenv("VAULT_ADDR", "http://127.0.0.1:8200")
				t.Setenv("VAULT_APP_ROLE", "jenkins")
				t.Setenv("VAULT_TOKEN", "root")
			},
			opts: []option.KeyFunc{
				option.WithKeyPrefixer(prefix),
				option.WithKeyCaser(option.Uppercase),
			},
			secretValue: map[string]interface{}{
				"user": "admin",
				"pass": "@dm|n",
			},
			cleanUp: func(ctx context.Context, client *api.Client, mountPath, secretPath string) {
				_ = client.KVv2(mountPath).Delete(ctx, secretPath)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt.envFunc()

		//////
		// Create a temp, new secret for testing purposes.
		//////

		ln, client := createTestVault(t)
		defer ln.Close()

		// // Setup vault, and create a raw client.
		// client, err := api.NewClient(api.DefaultConfig())
		// if err != nil {
		// 	t.Fatalf("unable to initialize Vault client: %s", err)
		// }

		if tt.credsAPI != nil {
			if tt.credsAPI.Address != "" {
				if err := client.SetAddress(tt.credsAPI.Address); err != nil {
					t.Fatalf("unable to set Vault address: %s", err)
				}
			}

			if tt.credsAPI.Token != "" {
				client.SetToken(tt.credsAPI.Token)
			}
		}

		//////
		// Ensure cleanup.
		//////
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			tt.cleanUp(
				ctx,
				client,
				tt.secretAPI.MountPath,
				tt.secretAPI.SecretPath,
			)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
				if err := got.Load(context.Background(), tt.opts...); (err != nil) != tt.wantErr {
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
