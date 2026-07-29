package provider

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/internal/logging"
)

//////
// Tests.
//////

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		override  bool
		rawValue  bool
		wantErr   bool
	}{
		{
			name:      "valid name",
			inputName: "provider",
		},
		{
			name:      "minimum name length",
			inputName: "abc",
			override:  true,
			rawValue:  true,
		},
		{
			name:      "maximum name length",
			inputName: strings.Repeat("a", 50),
		},
		{
			name:      "empty name",
			inputName: "",
			wantErr:   true,
		},
		{
			name:      "name below minimum length",
			inputName: "ab",
			wantErr:   true,
		},
		{
			name:      "name above maximum length",
			inputName: strings.Repeat("a", 51),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.inputName, tt.override, tt.rawValue)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.inputName, got.Name)
			assert.Equal(t, tt.override, got.Override)
			assert.Equal(t, tt.rawValue, got.RawValue)
			assert.NotNil(t, got.Logger)
		})
	}
}

func TestProviderAccessors(t *testing.T) {
	logger := logging.Get().Child("provider-accessors")

	tests := []struct {
		name         string
		provider     *Provider
		wantName     string
		wantLogger   *logging.Logger
		wantOverride bool
		wantRawValue bool
	}{
		{
			name: "all options enabled",
			provider: &Provider{
				Logger:   logger,
				Name:     "enabled",
				Override: true,
				RawValue: true,
			},
			wantName:     "enabled",
			wantLogger:   logger,
			wantOverride: true,
			wantRawValue: true,
		},
		{
			name: "zero-value options",
			provider: &Provider{
				Logger: logger,
				Name:   "",
			},
			wantName:   "",
			wantLogger: logger,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantName, tt.provider.GetName())
			assert.Same(t, tt.wantLogger, tt.provider.GetLogger())
			assert.Equal(t, tt.wantOverride, tt.provider.GetOverride())
			assert.Equal(t, tt.wantRawValue, tt.provider.GetRawValue())
		})
	}
}

func TestProviderExportToStruct(t *testing.T) {
	const endpointEnvVar = "CONFIGURER_PROVIDER_TEST_ENDPOINT"

	t.Setenv(endpointEnvVar, "https://example.test/config")

	type config struct {
		Endpoint string `env:"CONFIGURER_PROVIDER_TEST_ENDPOINT" validate:"required"`
		Retries  int    `default:"3"`
	}

	tests := []struct {
		name    string
		target  any
		want    *config
		wantErr bool
	}{
		{
			name:   "exports environment and defaults",
			target: &config{},
			want: &config{
				Endpoint: "https://example.test/config",
				Retries:  3,
			},
		},
		{
			name:    "rejects non-pointer target",
			target:  config{},
			wantErr: true,
		},
	}

	p, err := New("provider", false, false)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.ExportToStruct(tt.target)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, tt.target)
		})
	}
}
