//go:build unix

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/awssm"
	"github.com/thalesfsp/configurer/awsssm"
	"github.com/thalesfsp/configurer/azkv"
	"github.com/thalesfsp/configurer/doppler"
	"github.com/thalesfsp/configurer/gcpsm"
	"github.com/thalesfsp/configurer/noop"
	"github.com/thalesfsp/configurer/onepassword"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/configurer/vault"
)

const (
	cliExecuteHelperEnv      = "CONFIGURER_CLI_EXECUTE_HELPER"
	cliFakeExpectedInputEnv  = "CONFIGURER_CLI_FAKE_EXPECTED_INPUT"
	cliFakeProvidersEnv      = "CONFIGURER_CLI_FAKE_PROVIDERS"
	cliFakeExpectedRegionEnv = "CONFIGURER_CLI_FAKE_EXPECTED_REGION"
	cliStdinEnv              = "CONFIGURER_CLI_TEST_STDIN"
	cliUseWrapperEnv         = "CONFIGURER_CLI_USE_EXECUTE_WRAPPER"
)

//////
// Cobra execution with local providers.
//////

func TestCLIFlagBindingAndLocalProviders(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*testing.T) ([]string, map[string]string, func(*testing.T, string))
		wantExitCode int
		wantOutput   string
	}{
		{
			name: "happy path load dotenv binds transformations dump and child args",
			setup: func(t *testing.T) ([]string, map[string]string, func(*testing.T, string)) {
				t.Helper()

				envFile := filepath.Join(t.TempDir(), "fixture.env")
				dumpFile := filepath.Join(t.TempDir(), "loaded.json")
				require.NoError(t, os.WriteFile(
					envFile,
					[]byte("loaded_value=from-dotenv\n"),
					0o600,
				))

				args := []string{
					"--flush-interval=1ms",
					"load",
					"--override",
					"--dump", dumpFile,
					"--key-caser", "upper",
					"--key-prefixer", "PREFIX_",
					"--key-suffixer", "_SUFFIX",
					"dotenv",
					"--files", envFile,
					"--",
					"/bin/sh",
					"-c",
					`printf "%s" "$PREFIX_LOADED_VALUE_SUFFIX"`,
				}

				verify := func(t *testing.T, _ string) {
					t.Helper()

					dump, err := os.ReadFile(dumpFile)
					require.NoError(t, err)
					assert.JSONEq(
						t,
						`{"PREFIX_LOADED_VALUE_SUFFIX":"from-dotenv"}`,
						string(dump),
					)
				}

				return args, nil, verify
			},
			wantOutput: "from-dotenv",
		},
		{
			name: "happy path load noop binds prefix and forwards environment",
			setup: func(t *testing.T) ([]string, map[string]string, func(*testing.T, string)) {
				t.Helper()

				dumpFile := filepath.Join(t.TempDir(), "noop.json")
				args := []string{
					"--flush-interval=1ms",
					"load",
					"--dump", dumpFile,
					"--key-caser", "upper",
					"--key-prefixer", "COPIED_",
					"--key-suffixer", "_SUFFIX",
					"noop",
					"--",
					"/bin/sh",
					"-c",
					`printf "%s" "$COPIED_CONFIGURER_NOOP_INPUT_SUFFIX"`,
				}

				return args, map[string]string{
					"CONFIGURER_NOOP_INPUT": "from-noop",
				}, nil
			},
			wantOutput: "from-noop",
		},
		{
			name: "happy path load text binds format and stdin",
			setup: func(t *testing.T) ([]string, map[string]string, func(*testing.T, string)) {
				t.Helper()

				args := []string{
					"--flush-interval=1ms",
					"load",
					"--override",
					"text",
					"--format", "env",
					"--",
					"/bin/sh",
					"-c",
					`printf "%s" "$CONFIGURER_TEXT_CLI"`,
				}

				return args, map[string]string{
					cliStdinEnv: "CONFIGURER_TEXT_CLI=from-text\n",
				}, nil
			},
			wantOutput: "from-text",
		},
		{
			name: "bad path load text rejects unsupported format",
			setup: func(t *testing.T) ([]string, map[string]string, func(*testing.T, string)) {
				t.Helper()

				return []string{
						"--flush-interval=1ms",
						"load",
						"text",
						"--format", "xml",
					}, map[string]string{
						cliStdinEnv: "<value />",
					}, nil
			},
			wantExitCode: 1,
			wantOutput:   "invalid format",
		},
		{
			name: "bad path load dotenv missing fixture",
			setup: func(t *testing.T) ([]string, map[string]string, func(*testing.T, string)) {
				t.Helper()

				return []string{
					"--flush-interval=1ms",
					"load",
					"dotenv",
					"--files", filepath.Join(t.TempDir(), "missing.env"),
				}, nil, nil
			},
			wantExitCode: 1,
			wantOutput:   "read path",
		},
		{
			name: "happy path write dotenv binds source and target",
			setup: func(t *testing.T) ([]string, map[string]string, func(*testing.T, string)) {
				t.Helper()

				sourceFile := filepath.Join(t.TempDir(), "source.env")
				targetFile := filepath.Join(t.TempDir(), "target.env")
				require.NoError(t, os.WriteFile(
					sourceFile,
					[]byte("WRITE_KEY=write-value\n"),
					0o600,
				))

				args := []string{
					"write",
					"--source", sourceFile,
					"dotenv",
					"--target", targetFile,
				}

				verify := func(t *testing.T, _ string) {
					t.Helper()

					written, err := os.ReadFile(targetFile)
					require.NoError(t, err)
					assert.Contains(t, string(written), `WRITE_KEY="write-value"`)
				}

				return args, nil, verify
			},
		},
		{
			name: "bad path write dotenv missing source",
			setup: func(t *testing.T) ([]string, map[string]string, func(*testing.T, string)) {
				t.Helper()

				return []string{
					"write",
					"--source", filepath.Join(t.TempDir(), "missing.env"),
					"dotenv",
					"--target", filepath.Join(t.TempDir(), "target.env"),
				}, nil, nil
			},
			wantExitCode: 1,
			wantOutput:   "no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, environment, verify := tt.setup(t)
			output, err := runCLIHelper(t, t.TempDir(), environment, false, args...)

			assertProcessExitCode(t, tt.wantExitCode, err, output)
			assert.Contains(t, output, tt.wantOutput)

			if verify != nil {
				verify(t, output)
			}
		})
	}
}

//////
// Safe validation paths for remote-backed commands.
//////

func TestCLIProviderSuccessPathsUseLocalFakes(t *testing.T) {
	sourceFile := filepath.Join(t.TempDir(), "source.env")
	require.NoError(t, os.WriteFile(sourceFile, []byte("KEY=value\n"), 0o600))

	tests := []struct {
		name        string
		args        []string
		environment map[string]string
		provider    string
		wantInput   map[string]interface{}
	}{
		{
			name: "awssm load region-only configuration",
			args: []string{
				"--flush-interval=1ms",
				"load", "awssm",
				"--region", "us-west-2",
				"--secret-name", "secret",
			},
			environment: map[string]string{
				cliFakeExpectedRegionEnv: "us-west-2",
			},
			provider: "awssm",
			wantInput: map[string]interface{}{
				"config":      fakeAWSConfig("us-west-2", "", "", ""),
				"secretNames": []string{"secret"},
			},
		},
		{
			name: "gcpsm write binds project and secret",
			args: []string{
				"write", "--source", sourceFile,
				"gcpsm",
				"--project-id", "gcp-project",
				"--secret-name", "app-secret",
			},
			provider: "gcpsm",
			wantInput: map[string]interface{}{
				"projectID":   "gcp-project",
				"secretNames": []string{"app-secret"},
			},
		},
		{
			name: "gcpsm load binds project secret and alias",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"--override",
				"--rawValue",
				"--key-caser", "upper",
				"--key-prefixer", "PREFIX_",
				"--key-suffixer", "_SUFFIX",
				"gsm",
				"--project-id", "gcp-project",
				"--secret-name", "app-secret",
			},
			provider: "gcpsm",
			wantInput: map[string]interface{}{
				"projectID":   "gcp-project",
				"secretNames": []string{"app-secret"},
			},
		},
		{
			name: "gcpsm load binds environment fallbacks",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"gcpsm",
			},
			environment: map[string]string{
				"GCP_PROJECT_ID":    "environment-project",
				"GCPSM_SECRET_NAME": "environment-secret",
			},
			provider: "gcpsm",
			wantInput: map[string]interface{}{
				"projectID":   "environment-project",
				"secretNames": []string{"environment-secret"},
			},
		},
		{
			name: "gcpsm load binds Google Cloud project fallback",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"gcpsm",
			},
			environment: map[string]string{
				"GCPSM_SECRET_NAME":    "environment-secret",
				"GOOGLE_CLOUD_PROJECT": "google-environment-project",
			},
			provider: "gcpsm",
			wantInput: map[string]interface{}{
				"projectID":   "google-environment-project",
				"secretNames": []string{"environment-secret"},
			},
		},
		{
			name: "awssm load profile transformations and dump",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"--override",
				"--rawValue",
				"--dump", filepath.Join(t.TempDir(), "awssm.json"),
				"--key-caser", "upper",
				"--key-prefixer", "PREFIX_",
				"--key-suffixer", "_SUFFIX",
				"awssm",
				"--region", "us-east-1",
				"--profile", "profile",
				"--secret-name", "secret",
			},
			provider: "awssm",
			wantInput: map[string]interface{}{
				"config":      fakeAWSConfig("us-east-1", "profile", "", ""),
				"secretNames": []string{"secret"},
			},
		},
		{
			name: "awssm load access-key configuration",
			args: []string{
				"--flush-interval=1ms",
				"load", "awssm",
				"--region", "us-east-1",
				"--access-key", "access",
				"--secret-key", "secret-key",
				"--secret-name", "secret",
			},
			provider: "awssm",
			wantInput: map[string]interface{}{
				"config":      fakeAWSConfig("us-east-1", "", "access", "secret-key"),
				"secretNames": []string{"secret"},
			},
		},
		{
			name: "doppler load binds flags and load options",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"--override",
				"--rawValue",
				"doppler",
				"--token", "dp.pt.token",
				"--project", "project",
				"--config", "development",
			},
			provider: "doppler",
			wantInput: map[string]interface{}{
				"config": map[string]interface{}{
					"token":   "dp.pt.token",
					"project": "project",
					"config":  "development",
				},
				"override": true,
				"rawValue": true,
			},
		},
		{
			name: "doppler load alias binds environment fallbacks for service token",
			args: []string{
				"--flush-interval=1ms",
				"load", "dp",
			},
			environment: map[string]string{
				"DOPPLER_TOKEN": "dp.st.token",
			},
			provider: "doppler",
			wantInput: map[string]interface{}{
				"config": map[string]interface{}{
					"token":   "dp.st.token",
					"project": "",
					"config":  "",
				},
				"override": false,
				"rawValue": false,
			},
		},
		{
			name: "onepassword load alias binds flags and load options",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"--override",
				"--rawValue",
				"op",
				"--host", "https://connect.example.test",
				"--token", "token",
				"--vault", "Production",
				"--item", "Application",
			},
			provider: "onepassword",
			wantInput: map[string]interface{}{
				"config": map[string]interface{}{
					"host":  "https://connect.example.test",
					"token": "token",
					"vault": "Production",
					"item":  "Application",
				},
				"override": true,
				"rawValue": true,
			},
		},
		{
			name: "onepassword load binds environment fallbacks",
			args: []string{
				"--flush-interval=1ms",
				"load", "onepassword",
			},
			environment: map[string]string{
				"OP_CONNECT_HOST":  "https://environment-connect.example.test",
				"OP_CONNECT_TOKEN": "environment-token",
				"OP_VAULT":         "Environment Vault",
				"OP_ITEM":          "Environment Item",
			},
			provider: "onepassword",
			wantInput: map[string]interface{}{
				"config": map[string]interface{}{
					"host":  "https://environment-connect.example.test",
					"token": "environment-token",
					"vault": "Environment Vault",
					"item":  "Environment Item",
				},
				"override": false,
				"rawValue": false,
			},
		},
		{
			name: "awsssm load path flags transformations and dump",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"--override",
				"--rawValue",
				"--dump", filepath.Join(t.TempDir(), "awsssm.yaml"),
				"--key-caser", "lower",
				"--key-prefixer", "prefix_",
				"--key-suffixer", "_suffix",
				"awsssm",
				"--region", "us-east-1",
				"--profile", "profile",
				"--path", "/app",
				"--recursive=false",
				"--no-decrypt",
			},
			provider: "awsssm",
			wantInput: map[string]interface{}{
				"config":         fakeAWSConfig("us-east-1", "profile", "", ""),
				"parameterNames": nil,
				"path":           "/app",
				"recursive":      false,
				"withDecryption": false,
			},
		},
		{
			name: "awsssm load parameter-name access-key configuration",
			args: []string{
				"--flush-interval=1ms",
				"load", "awsssm",
				"--region", "us-east-1",
				"--access-key", "access",
				"--secret-key", "secret-key",
				"--parameter-name", "/app/key",
			},
			provider: "awsssm",
			wantInput: map[string]interface{}{
				"config":         fakeAWSConfig("us-east-1", "", "access", "secret-key"),
				"parameterNames": []string{"/app/key"},
				"path":           "",
				"recursive":      true,
				"withDecryption": true,
			},
		},
		{
			name: "azkv load alias binds vault and multiple secret names",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"--override",
				"--rawValue",
				"akv",
				"--vault-url", "https://azure-vault.example.test",
				"--secret-name", "first-secret",
				"--secret-name", "second-secret",
			},
			provider: "azkv",
			wantInput: map[string]interface{}{
				"vaultURL":    "https://azure-vault.example.test",
				"secretNames": []string{"first-secret", "second-secret"},
				"override":    true,
				"rawValue":    true,
			},
		},
		{
			name: "azkv load binds environment fallbacks",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"azkv",
			},
			environment: map[string]string{
				"AZURE_KEY_VAULT_URL":          "https://environment-vault.example.test",
				"AZURE_KEY_VAULT_SECRET_NAMES": "environment-one,environment-two",
			},
			provider: "azkv",
			wantInput: map[string]interface{}{
				"vaultURL":    "https://environment-vault.example.test",
				"secretNames": []string{"environment-one", "environment-two"},
				"override":    false,
				"rawValue":    false,
			},
		},
		{
			name: "vault load token flags transformations and dump",
			args: []string{
				"--flush-interval=1ms",
				"load",
				"--override",
				"--rawValue",
				"--dump", filepath.Join(t.TempDir(), "vault.yml"),
				"--key-caser", "upper",
				"--key-prefixer", "PREFIX_",
				"--key-suffixer", "_SUFFIX",
				"vault",
				"--address", "https://vault.example.test",
				"--namespace", "namespace",
				"--mount-path", "secret",
				"--secret-path", "app/config",
				"--token", "token",
				"--app-role", "role",
				"--role-id", "role-id",
				"--secret-id", "secret-id",
			},
			provider:  "vault",
			wantInput: fakeVaultInput(),
		},
		{
			name: "awssm write profile configuration",
			args: []string{
				"write", "--source", sourceFile,
				"awssm",
				"--region", "us-east-1",
				"--profile", "profile",
				"--secret-name", "secret",
			},
			provider: "awssm",
			wantInput: map[string]interface{}{
				"config":      fakeAWSConfig("us-east-1", "profile", "", ""),
				"secretNames": []string{"secret"},
			},
		},
		{
			name: "awssm write access-key configuration",
			args: []string{
				"write", "--source", sourceFile,
				"awssm",
				"--region", "us-east-1",
				"--access-key", "access",
				"--secret-key", "secret-key",
				"--secret-name", "secret",
			},
			provider: "awssm",
			wantInput: map[string]interface{}{
				"config":      fakeAWSConfig("us-east-1", "", "access", "secret-key"),
				"secretNames": []string{"secret"},
			},
		},
		{
			name: "doppler write binds flags",
			args: []string{
				"write", "--source", sourceFile,
				"doppler",
				"--token", "dp.pt.token",
				"--project", "project",
				"--config", "development",
			},
			provider: "doppler",
			wantInput: map[string]interface{}{
				"config": map[string]interface{}{
					"token":   "dp.pt.token",
					"project": "project",
					"config":  "development",
				},
				"override": false,
				"rawValue": false,
			},
		},
		{
			name: "doppler write binds environment fallbacks",
			args: []string{
				"write", "--source", sourceFile,
				"doppler",
			},
			environment: map[string]string{
				"DOPPLER_TOKEN":   "dp.pt.environment",
				"DOPPLER_PROJECT": "environment-project",
				"DOPPLER_CONFIG":  "production",
			},
			provider: "doppler",
			wantInput: map[string]interface{}{
				"config": map[string]interface{}{
					"token":   "dp.pt.environment",
					"project": "environment-project",
					"config":  "production",
				},
				"override": false,
				"rawValue": false,
			},
		},
		{
			name: "onepassword write alias binds flags",
			args: []string{
				"write", "--source", sourceFile,
				"op",
				"--host", "https://connect.example.test",
				"--token", "token",
				"--vault", "Production",
				"--item", "Application",
			},
			provider: "onepassword",
			wantInput: map[string]interface{}{
				"config": map[string]interface{}{
					"host":  "https://connect.example.test",
					"token": "token",
					"vault": "Production",
					"item":  "Application",
				},
				"override": false,
				"rawValue": false,
			},
		},
		{
			name: "onepassword write binds environment fallbacks",
			args: []string{
				"write", "--source", sourceFile,
				"onepassword",
			},
			environment: map[string]string{
				"OP_CONNECT_HOST":  "https://environment-connect.example.test",
				"OP_CONNECT_TOKEN": "environment-token",
				"OP_VAULT":         "Environment Vault",
				"OP_ITEM":          "Environment Item",
			},
			provider: "onepassword",
			wantInput: map[string]interface{}{
				"config": map[string]interface{}{
					"host":  "https://environment-connect.example.test",
					"token": "environment-token",
					"vault": "Environment Vault",
					"item":  "Environment Item",
				},
				"override": false,
				"rawValue": false,
			},
		},
		{
			name: "awsssm write profile configuration",
			args: []string{
				"write", "--source", sourceFile,
				"awsssm",
				"--region", "us-east-1",
				"--profile", "profile",
				"--path", "/app",
			},
			provider: "awsssm",
			wantInput: map[string]interface{}{
				"config":         fakeAWSConfig("us-east-1", "profile", "", ""),
				"parameterNames": nil,
				"path":           "/app",
				"recursive":      false,
				"withDecryption": true,
			},
		},
		{
			name: "awsssm write access-key configuration",
			args: []string{
				"write", "--source", sourceFile,
				"awsssm",
				"--region", "us-east-1",
				"--access-key", "access",
				"--secret-key", "secret-key",
				"--path", "/app",
			},
			provider: "awsssm",
			wantInput: map[string]interface{}{
				"config":         fakeAWSConfig("us-east-1", "", "access", "secret-key"),
				"parameterNames": nil,
				"path":           "/app",
				"recursive":      false,
				"withDecryption": true,
			},
		},
		{
			name: "azkv write binds vault URL",
			args: []string{
				"write", "--source", sourceFile,
				"azkv",
				"--vault-url", "https://azure-vault.example.test",
			},
			provider: "azkv",
			wantInput: map[string]interface{}{
				"vaultURL":    "https://azure-vault.example.test",
				"secretNames": nil,
				"override":    false,
				"rawValue":    false,
			},
		},
		{
			name: "azkv write binds vault URL environment fallback",
			args: []string{
				"write", "--source", sourceFile,
				"azkv",
			},
			environment: map[string]string{
				"AZURE_KEY_VAULT_URL": "https://environment-vault.example.test",
			},
			provider: "azkv",
			wantInput: map[string]interface{}{
				"vaultURL":    "https://environment-vault.example.test",
				"secretNames": nil,
				"override":    false,
				"rawValue":    false,
			},
		},
		{
			name: "vault write binds authentication and secret flags",
			args: []string{
				"write", "--source", sourceFile,
				"vault",
				"--address", "https://vault.example.test",
				"--namespace", "namespace",
				"--mount-path", "secret",
				"--secret-path", "app/config",
				"--token", "token",
				"--app-role", "role",
				"--role-id", "role-id",
				"--secret-id", "secret-id",
			},
			provider:  "vault",
			wantInput: fakeVaultInput(),
		},
		{
			name: "github write binds all local write options",
			args: []string{
				"write", "--source", sourceFile,
				"github",
				"--owner", "owner",
				"--repo", "repo",
				"--environment", "production",
				"--variable",
				"--target", "actions",
				"--httpVerb", "PUT",
			},
			provider: "github",
			wantInput: map[string]interface{}{
				"owner":      "owner",
				"repository": "repo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			environment := map[string]string{
				cliFakeProvidersEnv: "1",
				cliFakeExpectedInputEnv: fakeProviderInput(
					tt.provider,
					tt.wantInput,
				),
			}
			for key, value := range tt.environment {
				environment[key] = value
			}

			output, err := runCLIHelper(t, t.TempDir(), environment, false, tt.args...)

			assertProcessExitCode(t, 0, err, output)
		})
	}
}

func TestCLIProviderValidationWithoutExternalServices(t *testing.T) {
	sourceFile := filepath.Join(t.TempDir(), "source.env")
	require.NoError(t, os.WriteFile(sourceFile, []byte("KEY=value\n"), 0o600))

	tests := []struct {
		name       string
		args       []string
		wantOutput string
	}{
		{
			name: "bad path awssm load rejects profile with access key",
			args: []string{
				"load", "awssm",
				"--secret-name", "secret",
				"--profile", "profile",
				"--access-key", "access",
			},
			wantOutput: "if --profile is specified",
		},
		{
			name: "bad path awssm load requires region without profile",
			args: []string{
				"load", "awssm",
				"--secret-name", "secret",
			},
			wantOutput: "if --profile is not specified",
		},
		{
			name: "bad path awssm load rejects secret key without access key",
			args: []string{
				"load", "awssm",
				"--secret-name", "secret",
				"--region", "us-east-1",
				"--secret-key", "secret-key",
			},
			wantOutput: "if --secret-key is specified",
		},
		{
			name:       "bad path gcpsm load requires project",
			args:       []string{"load", "gcpsm", "--secret-name", "secret"},
			wantOutput: "--project-id is required",
		},
		{
			name:       "bad path gcpsm load requires secret",
			args:       []string{"load", "gcpsm", "--project-id", "project"},
			wantOutput: "--secret-name is required",
		},
		{
			name: "bad path awsssm load rejects profile with access key",
			args: []string{
				"load", "awsssm",
				"--path", "/app",
				"--profile", "profile",
				"--access-key", "access",
			},
			wantOutput: "if --profile is specified",
		},
		{
			name: "bad path awsssm load requires region without profile",
			args: []string{
				"load", "awsssm",
				"--path", "/app",
			},
			wantOutput: "if --profile is not specified",
		},
		{
			name: "bad path awsssm load rejects secret key without access key",
			args: []string{
				"load", "awsssm",
				"--path", "/app",
				"--region", "us-east-1",
				"--secret-key", "secret-key",
			},
			wantOutput: "if --secret-key is specified",
		},
		{
			name: "bad path awsssm load requires path or parameter name",
			args: []string{
				"load", "awsssm",
				"--region", "us-east-1",
			},
			wantOutput: "either --path or --parameter-name",
		},
		{
			name: "bad path awssm write rejects profile with access key",
			args: []string{
				"write", "--source", sourceFile,
				"awssm",
				"--secret-name", "secret",
				"--profile", "profile",
				"--access-key", "access",
			},
			wantOutput: "if --profile is specified",
		},
		{
			name: "bad path awssm write requires region without profile",
			args: []string{
				"write", "--source", sourceFile,
				"awssm",
				"--secret-name", "secret",
			},
			wantOutput: "if --profile is not specified",
		},
		{
			name: "bad path awssm write rejects secret key without access key",
			args: []string{
				"write", "--source", sourceFile,
				"awssm",
				"--secret-name", "secret",
				"--region", "us-east-1",
				"--secret-key", "secret-key",
			},
			wantOutput: "if --secret-key is specified",
		},
		{
			name: "bad path awssm write rejects access key without secret key",
			args: []string{
				"write", "--source", sourceFile,
				"awssm",
				"--secret-name", "secret",
				"--region", "us-east-1",
				"--access-key", "access",
			},
			wantOutput: "if --access-key is specified",
		},
		{
			name: "bad path gcpsm write requires project",
			args: []string{
				"write", "--source", sourceFile,
				"gcpsm",
				"--secret-name", "secret",
			},
			wantOutput: "--project-id is required",
		},
		{
			name: "bad path gcpsm write requires secret",
			args: []string{
				"write", "--source", sourceFile,
				"gcpsm",
				"--project-id", "project",
			},
			wantOutput: "--secret-name is required",
		},
		{
			name: "bad path awsssm write rejects profile with access key",
			args: []string{
				"write", "--source", sourceFile,
				"awsssm",
				"--path", "/app",
				"--profile", "profile",
				"--access-key", "access",
			},
			wantOutput: "if --profile is specified",
		},
		{
			name: "bad path awsssm write requires region without profile",
			args: []string{
				"write", "--source", sourceFile,
				"awsssm",
				"--path", "/app",
			},
			wantOutput: "if --profile is not specified",
		},
		{
			name: "bad path awsssm write rejects secret key without access key",
			args: []string{
				"write", "--source", sourceFile,
				"awsssm",
				"--path", "/app",
				"--region", "us-east-1",
				"--secret-key", "secret-key",
			},
			wantOutput: "if --secret-key is specified",
		},
		{
			name: "bad path awsssm write rejects access key without secret key",
			args: []string{
				"write", "--source", sourceFile,
				"awsssm",
				"--path", "/app",
				"--region", "us-east-1",
				"--access-key", "access",
			},
			wantOutput: "if --access-key is specified",
		},
		{
			name: "bad path awsssm write requires path",
			args: []string{
				"write", "--source", sourceFile,
				"awsssm",
				"--region", "us-east-1",
			},
			wantOutput: "--path is required",
		},
		{
			name: "bad path vault load validates required local configuration",
			args: []string{
				"load", "vault",
			},
			wantOutput: "invalid struct",
		},
		{
			name: "bad path azkv load requires vault URL",
			args: []string{
				"load", "azkv",
			},
			wantOutput: "VaultURL",
		},
		{
			name: "bad path azkv write requires vault URL",
			args: []string{
				"write", "--source", sourceFile,
				"azkv",
			},
			wantOutput: "VaultURL",
		},
		{
			name: "bad path vault write validates required local configuration",
			args: []string{
				"write", "--source", sourceFile,
				"vault",
			},
			wantOutput: "invalid struct",
		},
		{
			name: "bad path doppler load requires project and config for regular token",
			args: []string{
				"load", "doppler",
				"--token", "dp.pt.token",
				"--project", "project",
			},
			wantOutput: "project and config",
		},
		{
			name: "bad path doppler write requires token",
			args: []string{
				"write", "--source", sourceFile,
				"doppler",
				"--project", "project",
				"--config", "development",
			},
			wantOutput: "Token",
		},
		{
			name: "bad path github write requires token before network access",
			args: []string{
				"write", "--source", sourceFile,
				"github",
				"--owner", "owner",
				"--repo", "repo",
			},
			wantOutput: "GITHUB_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runCLIHelper(t, t.TempDir(), nil, false, tt.args...)

			assertProcessExitCode(t, 1, err, output)
			assert.Contains(t, output, tt.wantOutput)
		})
	}
}

func TestCLIStartValidation(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantOutput string
	}{
		{
			name:       "bad path destination missing",
			args:       []string{"bridge", "start"},
			wantOutput: "missing required flag --destination",
		},
		{
			name: "bad path server missing",
			args: []string{
				"bridge", "start",
				"--destination", "127.0.0.1:8080",
			},
			wantOutput: "missing required flag --server",
		},
		{
			name: "bad path source missing",
			args: []string{
				"bridge", "start",
				"--destination", "127.0.0.1:8080",
				"--server", "user@example.test",
			},
			wantOutput: "missing required flag --source",
		},
		{
			name: "bad path authentication missing",
			args: []string{
				"bridge", "start",
				"--destination", "127.0.0.1:8080",
				"--server", "user@example.test",
				"--source", "127.0.0.1:9090",
			},
			wantOutput: "missing required flag --key or --key-value",
		},
		{
			name: "bad path key modes are mutually exclusive",
			args: []string{
				"bridge", "start",
				"--destination", "127.0.0.1:8080",
				"--server", "user@example.test",
				"--source", "127.0.0.1:9090",
				"--key", "key-file",
				"--key-value", "key-value",
			},
			wantOutput: "or --key or --key-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runCLIHelper(t, t.TempDir(), nil, false, tt.args...)

			assertProcessExitCode(t, 1, err, output)
			assert.Contains(t, output, tt.wantOutput)
		})
	}
}

func TestExecuteWrapperErrorPath(t *testing.T) {
	output, err := runCLIHelper(
		t,
		t.TempDir(),
		nil,
		true,
		"definitely-not-a-command",
	)

	assertProcessExitCode(t, 1, err, output)
}

//////
// Parent command validation.
//////

func TestCLIParentCommandsRequireSubcommands(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantExitCode int
		wantOutput   string
	}{
		{
			name:         "happy path load help exits zero",
			args:         []string{"load", "--help"},
			wantExitCode: 0,
			wantOutput:   "Available Providers",
		},
		{
			name:         "happy path write help exits zero",
			args:         []string{"write", "--help"},
			wantExitCode: 0,
			wantOutput:   "Available Providers",
		},
		{
			name:         "bad path load rejects unknown provider",
			args:         []string{"load", "definitely-not-a-provider"},
			wantExitCode: 1,
			wantOutput:   `unknown command "definitely-not-a-provider" for "configurer load"`,
		},
		{
			name:         "bad path write rejects unknown provider",
			args:         []string{"write", "definitely-not-a-provider"},
			wantExitCode: 1,
			wantOutput:   `unknown command "definitely-not-a-provider" for "configurer write"`,
		},
		{
			name:         "edge path load requires a provider",
			args:         []string{"load"},
			wantExitCode: 1,
			wantOutput:   "subcommand required",
		},
		{
			name:         "edge path write requires a provider",
			args:         []string{"write"},
			wantExitCode: 1,
			wantOutput:   "subcommand required",
		},
		{
			name:         "edge path root requires a subcommand",
			wantExitCode: 1,
			wantOutput:   "subcommand required",
		},
		{
			name:         "edge path root help exits zero",
			args:         []string{"--help"},
			wantExitCode: 0,
			wantOutput:   "Available Commands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runCLIHelper(t, t.TempDir(), nil, false, tt.args...)

			assertProcessExitCode(t, tt.wantExitCode, err, output)
			assert.Contains(t, output, tt.wantOutput)
		})
	}
}

func TestCLIExecuteHelper(t *testing.T) {
	if os.Getenv(cliExecuteHelperEnv) == "" {
		return
	}

	rootCmd.SetArgs(cliArguments())

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)

	if os.Getenv(cliFakeProvidersEnv) != "" {
		installCLIProviderFakes()
	}

	if os.Getenv(cliUseWrapperEnv) != "" {
		Execute()

		return
	}

	err := rootCmd.Execute()
	_, _ = os.Stdout.Write(output.Bytes())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cliArguments() []string {
	for i, arg := range os.Args {
		if arg == "--" {
			return os.Args[i+1:]
		}
	}

	return nil
}

func runCLIHelper(
	t *testing.T,
	workDir string,
	environment map[string]string,
	useExecuteWrapper bool,
	args ...string,
) (string, error) {
	t.Helper()

	commandArgs := []string{
		"-test.run=^TestCLIExecuteHelper$",
		"--",
	}
	commandArgs = append(commandArgs, args...)

	command := exec.Command(os.Args[0], commandArgs...)
	command.Dir = workDir
	if stdin := environment[cliStdinEnv]; stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}

	overrides := map[string]string{
		cliExecuteHelperEnv:             "1",
		"AWS_ACCESS_KEY_ID":             "",
		"AWS_PROFILE":                   "",
		"AWS_REGION":                    "",
		"AWS_SECRET_ACCESS_KEY":         "",
		"AWSSM_SECRET_NAME":             "",
		"AWSSSM_PARAMETER_NAME":         "",
		"AWSSSM_PATH":                   "",
		"AZURE_KEY_VAULT_SECRET_NAMES":  "",
		"AZURE_KEY_VAULT_URL":           "",
		"CONFIGURER_BRIDGE_DESTINATION": "",
		"CONFIGURER_BRIDGE_KEY":         "",
		"CONFIGURER_BRIDGE_SERVER":      "",
		"CONFIGURER_BRIDGE_SOURCE":      "",
		"DOPPLER_CONFIG":                "",
		"DOPPLER_PROJECT":               "",
		"DOPPLER_TOKEN":                 "",
		"GITHUB_TOKEN":                  "",
		"GCP_PROJECT_ID":                "",
		"GCPSM_SECRET_NAME":             "",
		"GOOGLE_CLOUD_PROJECT":          "",
		"OP_CONNECT_HOST":               "",
		"OP_CONNECT_TOKEN":              "",
		"OP_ITEM":                       "",
		"OP_VAULT":                      "",
		"VAULT_ADDR":                    "",
		"VAULT_APP_ROLE":                "",
		"VAULT_APP_ROLE_ID":             "",
		"VAULT_APP_SECRET_ID":           "",
		"VAULT_MOUNT_PATH":              "",
		"VAULT_NAMESPACE":               "",
		"VAULT_SECRET_PATH":             "",
		"VAULT_TOKEN":                   "",
	}

	if useExecuteWrapper {
		overrides[cliUseWrapperEnv] = "1"
	}

	for key, value := range environment {
		overrides[key] = value
	}

	command.Env = environmentWithOverrides(overrides)

	output, err := command.CombinedOutput()

	return string(output), err
}

func environmentWithOverrides(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))

	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, overridden := overrides[key]; overridden {
			continue
		}

		environment = append(environment, entry)
	}

	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}

	return environment
}

func installCLIProviderFakes() {
	newAZKVProvider = func(
		override, rawValue bool,
		config *azkv.Config,
	) (provider.IProvider, error) {
		if err := validateFakeProviderInput("azkv", map[string]interface{}{
			"vaultURL":    config.VaultURL,
			"secretNames": config.SecretNames,
			"override":    override,
			"rawValue":    rawValue,
		}); err != nil {
			return nil, err
		}

		return noop.New(override, rawValue)
	}

	newGCPSMProvider = func(
		override, rawValue bool,
		config *gcpsm.Config,
		secretInformation *gcpsm.SecretInformation,
	) (provider.IProvider, error) {
		if err := validateFakeProviderInput("gcpsm", map[string]interface{}{
			"projectID":   config.ProjectID,
			"secretNames": secretInformation.SecretNames,
		}); err != nil {
			return nil, err
		}

		return noop.New(override, rawValue)
	}

	newAWSSMProvider = func(
		override, rawValue bool,
		config *awssm.Config,
		secretInformation *awssm.SecretInformation,
	) (provider.IProvider, error) {
		if expectedRegion := os.Getenv(cliFakeExpectedRegionEnv); expectedRegion != "" &&
			config.Region != expectedRegion {
			return nil, fmt.Errorf("region binding: got %q, want %q", config.Region, expectedRegion)
		}

		if err := validateFakeProviderInput("awssm", map[string]interface{}{
			"config": fakeAWSConfig(
				config.Region,
				config.Profile,
				config.AccessKey,
				config.SecretKey,
			),
			"secretNames": secretInformation.SecretNames,
		}); err != nil {
			return nil, err
		}

		return noop.New(override, rawValue)
	}

	newAWSSSMProvider = func(
		override, rawValue bool,
		config *awsssm.Config,
		parameterInformation *awsssm.ParameterInformation,
	) (provider.IProvider, error) {
		if err := validateFakeProviderInput("awsssm", map[string]interface{}{
			"config": fakeAWSConfig(
				config.Region,
				config.Profile,
				config.AccessKey,
				config.SecretKey,
			),
			"parameterNames": parameterInformation.ParameterNames,
			"path":           parameterInformation.Path,
			"recursive":      parameterInformation.Recursive,
			"withDecryption": parameterInformation.WithDecryption,
		}); err != nil {
			return nil, err
		}

		return noop.New(override, rawValue)
	}

	newVaultProvider = func(
		override, rawValue bool,
		auth *vault.Auth,
		secretInformation *vault.SecretInformation,
	) (provider.IProvider, error) {
		if err := validateFakeProviderInput("vault", map[string]interface{}{
			"auth": map[string]interface{}{
				"address":   auth.Address,
				"appRole":   auth.AppRole,
				"namespace": auth.Namespace,
				"roleID":    auth.RoleID,
				"secretID":  auth.SecretID,
				"token":     auth.Token,
			},
			"secretInformation": map[string]interface{}{
				"mountPath":  secretInformation.MountPath,
				"secretPath": secretInformation.SecretPath,
			},
		}); err != nil {
			return nil, err
		}

		return noop.New(override, rawValue)
	}

	newGitHubProvider = func(
		override, rawValue bool,
		owner, repository string,
	) (provider.IProvider, error) {
		if err := validateFakeProviderInput("github", map[string]interface{}{
			"owner":      owner,
			"repository": repository,
		}); err != nil {
			return nil, err
		}

		return noop.New(override, rawValue)
	}

	newDopplerProvider = func(
		override, rawValue bool,
		config *doppler.Config,
	) (provider.IProvider, error) {
		if err := validateFakeProviderInput("doppler", map[string]interface{}{
			"config": map[string]interface{}{
				"token":   config.Token,
				"project": config.Project,
				"config":  config.Config,
			},
			"override": override,
			"rawValue": rawValue,
		}); err != nil {
			return nil, err
		}

		return noop.New(override, rawValue)
	}

	newOnePasswordProvider = func(
		override, rawValue bool,
		config *onepassword.Config,
	) (provider.IProvider, error) {
		if err := validateFakeProviderInput("onepassword", map[string]interface{}{
			"config": map[string]interface{}{
				"host":  config.Host,
				"token": config.Token,
				"vault": config.Vault,
				"item":  config.Item,
			},
			"override": override,
			"rawValue": rawValue,
		}); err != nil {
			return nil, err
		}

		return noop.New(override, rawValue)
	}
}

func fakeAWSConfig(region, profile, accessKey, secretKey string) map[string]interface{} {
	return map[string]interface{}{
		"accessKey": accessKey,
		"profile":   profile,
		"region":    region,
		"secretKey": secretKey,
	}
}

func fakeVaultInput() map[string]interface{} {
	return map[string]interface{}{
		"auth": map[string]interface{}{
			"address":   "https://vault.example.test",
			"appRole":   "role",
			"namespace": "namespace",
			"roleID":    "role-id",
			"secretID":  "secret-id",
			"token":     "token",
		},
		"secretInformation": map[string]interface{}{
			"mountPath":  "secret",
			"secretPath": "app/config",
		},
	}
}

func fakeProviderInput(kind string, values map[string]interface{}) string {
	data, err := json.Marshal(map[string]interface{}{
		"kind":   kind,
		"values": values,
	})
	if err != nil {
		panic(err)
	}

	return string(data)
}

func validateFakeProviderInput(kind string, values map[string]interface{}) error {
	expectedJSON := os.Getenv(cliFakeExpectedInputEnv)
	if expectedJSON == "" {
		return nil
	}

	actualJSON := fakeProviderInput(kind, values)

	var expected, actual interface{}
	if err := json.Unmarshal([]byte(expectedJSON), &expected); err != nil {
		return fmt.Errorf("decode expected fake provider input: %w", err)
	}

	if err := json.Unmarshal([]byte(actualJSON), &actual); err != nil {
		return fmt.Errorf("decode actual fake provider input: %w", err)
	}

	if !reflect.DeepEqual(expected, actual) {
		return fmt.Errorf("provider flag binding: got %s, want %s", actualJSON, expectedJSON)
	}

	return nil
}
