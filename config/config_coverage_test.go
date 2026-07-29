package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//////
// Test types.
//////

type coverageMarshalFailure struct{}

func (coverageMarshalFailure) MarshalYAML() (interface{}, error) {
	return nil, errors.New("unsupported test value")
}

type coverageConfig struct {
	Model       string                  `yaml:"model"`
	Unsupported *coverageMarshalFailure `yaml:"unsupported,omitempty"`
}

//////
// Happy paths.
//////

func TestLoadConfigurationWritesDefaults(t *testing.T) {
	tests := []struct {
		name     string
		appName  string
		filePath func(t *testing.T) string
	}{
		{
			name: "explicit missing file",
			filePath: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "config.yaml")
			},
		},
		{
			name:    "default home path",
			appName: "configurer-coverage",
			filePath: func(t *testing.T) string {
				t.Helper()

				home := t.TempDir()
				t.Setenv("HOME", home)

				return ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultConfig := &coverageConfig{Model: "default"}
			filePath := tt.filePath(t)

			got, err := LoadConfiguration(filePath, tt.appName, defaultConfig)
			require.NoError(t, err)
			assert.Same(t, defaultConfig, got)

			if filePath == "" {
				home, err := os.UserHomeDir()
				require.NoError(t, err)
				filePath = filepath.Join(home, ".config", tt.appName, "config.yaml")
			}

			data, err := os.ReadFile(filePath)
			require.NoError(t, err)
			assert.Equal(t, "model: default\n", string(data))

			info, err := os.Stat(filePath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(fileperm), info.Mode().Perm())
		})
	}
}

//////
// Validation and error paths.
//////

func TestLoadConfigurationErrors(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) (string, string, *coverageConfig)
		wantError string
	}{
		{
			name: "nil default configuration",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				return "unused.yaml", "", nil
			},
			wantError: "defaultConfiguration",
		},
		{
			name: "missing app name for default path",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				return "", "", &coverageConfig{Model: "default"}
			},
			wantError: "appName",
		},
		{
			name: "home directory unavailable",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				home, hadHome := os.LookupEnv("HOME")
				require.NoError(t, os.Unsetenv("HOME"))
				t.Cleanup(func() {
					if hadHome {
						require.NoError(t, os.Setenv("HOME", home))
					} else {
						require.NoError(t, os.Unsetenv("HOME"))
					}
				})

				return "", "configurer-coverage", &coverageConfig{Model: "default"}
			},
			wantError: "get home directory",
		},
		{
			name: "config directory cannot be created",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				homeFile := filepath.Join(t.TempDir(), "home-file")
				require.NoError(t, os.WriteFile(homeFile, []byte("not a directory"), 0o600))
				t.Setenv("HOME", homeFile)

				return "", "configurer-coverage", &coverageConfig{Model: "default"}
			},
			wantError: "create config directory",
		},
		{
			name: "config path is a directory",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				return t.TempDir(), "", &coverageConfig{Model: "default"}
			},
			wantError: "read config file",
		},
		{
			name: "malformed yaml",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				path := filepath.Join(t.TempDir(), "config.yaml")
				require.NoError(t, os.WriteFile(path, []byte("model: [unterminated\n"), 0o600))

				return path, "", &coverageConfig{Model: "default"}
			},
			wantError: "parse config file",
		},
		{
			name: "missing parent directory is unwritable",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				return filepath.Join(t.TempDir(), "missing", "config.yaml"), "", &coverageConfig{Model: "default"}
			},
			wantError: "write default config",
		},
		{
			name: "existing file is unreadable",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				path := filepath.Join(t.TempDir(), "config.yaml")
				require.NoError(t, os.WriteFile(path, []byte("model: custom\n"), 0o600))
				require.NoError(t, os.Chmod(path, 0))
				t.Cleanup(func() {
					require.NoError(t, os.Chmod(path, 0o600))
				})

				return path, "", &coverageConfig{Model: "default"}
			},
			wantError: "read config file",
		},
		{
			name: "empty file is not writable",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				path := filepath.Join(t.TempDir(), "config.yaml")
				require.NoError(t, os.WriteFile(path, nil, 0o600))
				require.NoError(t, os.Chmod(path, 0o400))
				t.Cleanup(func() {
					require.NoError(t, os.Chmod(path, 0o600))
				})

				return path, "", &coverageConfig{Model: "default"}
			},
			wantError: "write default config",
		},
		{
			name: "default cannot be marshaled",
			setup: func(t *testing.T) (string, string, *coverageConfig) {
				t.Helper()

				return filepath.Join(t.TempDir(), "config.yaml"), "", &coverageConfig{
					Model:       "default",
					Unsupported: &coverageMarshalFailure{},
				}
			},
			wantError: "marshal default config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, appName, defaultConfig := tt.setup(t)

			got, err := LoadConfiguration(filePath, appName, defaultConfig)
			require.Error(t, err)
			assert.Nil(t, got)
			assert.ErrorContains(t, err, tt.wantError)
		})
	}
}
