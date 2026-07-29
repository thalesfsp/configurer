package dotenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/option"
)

//////
// Test helpers.
//////

func writeFixture(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

//////
// Construction and validation.
//////

func TestNewValidationAndFlags(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		override  bool
		rawValue  bool
		wantError string
	}{
		{
			name:      "file path is required",
			wantError: "filePaths",
		},
		{
			name:     "default flags",
			files:    []string{"config.env"},
			override: false,
			rawValue: false,
		},
		{
			name:     "override and raw value flags",
			files:    []string{"config.env"},
			override: true,
			rawValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.override, tt.rawValue, tt.files...)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Nil(t, got)
				assert.ErrorContains(t, err, tt.wantError)

				return
			}

			require.NoError(t, err)
			dotEnv, ok := got.(*DotEnv)
			require.True(t, ok)
			assert.Equal(t, tt.override, dotEnv.GetOverride())
			assert.Equal(t, tt.rawValue, dotEnv.GetRawValue())
			assert.Equal(t, tt.files, dotEnv.FilePaths)
		})
	}
}

//////
// Load behavior.
//////

func TestLoadFixtures(t *testing.T) {
	tests := []struct {
		name      string
		contents  []string
		want      map[string]string
		wantError string
		missing   bool
	}{
		{
			name:     "single file",
			contents: []string{"CONFIGURER_DOTENV_ONE=first\n"},
			want: map[string]string{
				"CONFIGURER_DOTENV_ONE": "first",
			},
		},
		{
			name: "multiple files merge with later values winning",
			contents: []string{
				"CONFIGURER_DOTENV_ONE=first\nCONFIGURER_DOTENV_SHARED=old\n",
				"CONFIGURER_DOTENV_TWO=second\nCONFIGURER_DOTENV_SHARED=new\n",
			},
			want: map[string]string{
				"CONFIGURER_DOTENV_ONE":    "first",
				"CONFIGURER_DOTENV_TWO":    "second",
				"CONFIGURER_DOTENV_SHARED": "new",
			},
		},
		{
			name:      "missing file",
			missing:   true,
			wantError: "read path",
		},
		{
			name:      "malformed line",
			contents:  []string{"BROKEN LINE\n"},
			wantError: "read path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var files []string
			if tt.missing {
				files = []string{filepath.Join(t.TempDir(), "missing.env")}
			} else {
				for index, content := range tt.contents {
					files = append(files, writeFixture(t, "fixture-"+string(rune('a'+index))+".env", content))
				}
			}

			for key := range tt.want {
				t.Setenv(key, "")
			}

			dotEnv, err := New(true, false, files...)
			require.NoError(t, err)

			got, err := dotEnv.Load(context.Background())
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Nil(t, got)
				assert.ErrorContains(t, err, tt.wantError)

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

func TestLoadOverrideAndRawValueSemantics(t *testing.T) {
	const key = "CONFIGURER_DOTENV_SEMANTICS"

	tests := []struct {
		name     string
		existing string
		want     string
		override bool
		rawValue bool
	}{
		{
			name:     "existing value is preserved",
			existing: "from-environment",
			want:     "from-environment",
		},
		{
			name:     "override replaces existing value",
			existing: "from-environment",
			want:     "from-file",
			override: true,
		},
		{
			name:     "raw value uses Go quoted representation",
			want:     `"from-file"`,
			override: true,
			rawValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(key, tt.existing)
			path := writeFixture(t, "semantics.env", key+"=from-file\n")

			dotEnv, err := New(tt.override, tt.rawValue, path)
			require.NoError(t, err)

			got, err := dotEnv.Load(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.want, got[key])
			assert.Equal(t, tt.want, os.Getenv(key))
		})
	}
}

func TestLoadRejectsInvalidTransformedKey(t *testing.T) {
	path := writeFixture(t, "invalid-key.env", "VALID_KEY=value\n")
	dotEnv, err := New(true, false, path)
	require.NoError(t, err)

	got, err := dotEnv.Load(
		context.Background(),
		option.WithKeyReplacer(func(string) string { return "INVALID=KEY" }),
	)

	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorContains(t, err, "export INVALID=KEY env var")
}

//////
// Write behavior.
//////

func TestWrite(t *testing.T) {
	sentinel := errors.New("write option failed")

	tests := []struct {
		name      string
		values    map[string]interface{}
		opts      []option.WriteFunc
		pathCount int
		badPath   bool
		want      []string
		wantError string
	}{
		{
			name:      "values are required",
			pathCount: 1,
			wantError: "values",
		},
		{
			name:      "multiple files are rejected",
			values:    map[string]interface{}{"KEY": "value"},
			pathCount: 2,
			wantError: "only one file",
		},
		{
			name:      "option error is returned",
			values:    map[string]interface{}{"KEY": "value"},
			pathCount: 1,
			opts: []option.WriteFunc{
				func(*option.Write) error { return sentinel },
			},
			wantError: sentinel.Error(),
		},
		{
			name:      "unwritable path returns an error",
			values:    map[string]interface{}{"KEY": "value"},
			pathCount: 1,
			badPath:   true,
			wantError: "write path",
		},
		{
			name:      "writes converted values",
			values:    map[string]interface{}{"COUNT": 7, "ENABLED": true},
			pathCount: 1,
			opts: []option.WriteFunc{
				option.WithEnvironment("testing"),
				option.WithVariable(true),
			},
			want: []string{"COUNT=7", `ENABLED="true"`},
		},
		{
			name:      "empty map writes an empty file",
			values:    map[string]interface{}{},
			pathCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			files := make([]string, tt.pathCount)
			for index := range files {
				files[index] = filepath.Join(tempDir, "output-"+string(rune('a'+index))+".env")
			}
			if tt.badPath {
				files[0] = filepath.Join(tempDir, "missing", "output.env")
			}

			dotEnv, err := New(true, false, files...)
			require.NoError(t, err)

			err = dotEnv.Write(context.Background(), tt.values, tt.opts...)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantError)

				return
			}

			require.NoError(t, err)
			data, err := os.ReadFile(files[0])
			require.NoError(t, err)
			for _, expected := range tt.want {
				assert.Contains(t, string(data), expected)
			}
			if len(tt.want) == 0 {
				assert.Empty(t, strings.TrimSpace(string(data)))
			}
		})
	}
}
