package util

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//////
// Parse helpers.
//////

func TestParseAPIs(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		content string
		want    map[string]any
		wantErr bool
	}{
		{
			name:    "env",
			format:  "env",
			content: "NAME=configurer\n",
			want:    map[string]any{"NAME": "configurer"},
		},
		{
			name:    "json",
			format:  "json",
			content: `{"NAME":"configurer"}`,
			want:    map[string]any{"NAME": "configurer"},
		},
		{
			name:    "yaml",
			format:  "yaml",
			content: "NAME: configurer\n",
			want:    map[string]any{"NAME": "configurer"},
		},
		{
			name:    "yml alias",
			format:  "yml",
			content: "NAME: configurer\n",
			want:    map[string]any{"NAME": "configurer"},
		},
		{
			name:    "toml",
			format:  "toml",
			content: "NAME = \"configurer\"\n",
			want:    map[string]any{"NAME": "configurer"},
		},
		{
			name:    "unknown format",
			format:  "xml",
			content: "<NAME>configurer</NAME>",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			t.Run("ParseContent", func(t *testing.T) {
				got, err := ParseContent(ctx, tt.format, strings.NewReader(tt.content))
				assertParseResult(t, got, err, tt.want, tt.wantErr)
			})

			t.Run("ParseFromText", func(t *testing.T) {
				got, err := ParseFromText(ctx, tt.format, tt.content)
				assertParseResult(t, got, err, tt.want, tt.wantErr)
			})

			t.Run("ParseFile", func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "config."+tt.format)
				require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

				file, err := os.Open(path)
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, file.Close())
				})

				got, err := ParseFile(ctx, file)
				assertParseResult(t, got, err, tt.want, tt.wantErr)
			})
		})
	}
}

func TestParseContentErrors(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		content string
	}{
		{
			name:    "env",
			format:  "env",
			content: "MISSING_SEPARATOR",
		},
		{
			name:    "json",
			format:  "json",
			content: `{"unterminated":`,
		},
		{
			name:    "yaml",
			format:  "yaml",
			content: "value: [unterminated",
		},
		{
			name:    "yml alias",
			format:  "yml",
			content: "value: [unterminated",
		},
		{
			name:    "toml",
			format:  "toml",
			content: `value = "unterminated`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseContent(context.Background(), tt.format, strings.NewReader(tt.content))

			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func assertParseResult(
	t *testing.T,
	got map[string]any,
	err error,
	want map[string]any,
	wantErr bool,
) {
	t.Helper()

	if wantErr {
		require.Error(t, err)
		assert.Nil(t, got)

		return
	}

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

//////
// General helpers.
//////

func TestGetValidator(t *testing.T) {
	assert.NotNil(t, GetValidator())
}

func TestGetZeroControlChar(t *testing.T) {
	tests := []struct {
		name     string
		override string
		want     string
	}{
		{
			name: "default",
			want: "zero",
		},
		{
			name:     "environment override",
			override: "clear",
			want:     "clear",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CONFIGURER_ZERO_CONTROL_CHAR", tt.override)
			assert.Equal(t, tt.want, GetZeroControlChar())
		})
	}
}
