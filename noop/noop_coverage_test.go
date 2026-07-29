package noop

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/validation"
)

//////
// Construction and validation.
//////

func TestNewFlags(t *testing.T) {
	tests := []struct {
		name     string
		override bool
		rawValue bool
	}{
		{name: "default flags"},
		{name: "override enabled", override: true},
		{name: "raw value enabled", rawValue: true},
		{name: "all flags enabled", override: true, rawValue: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.override, tt.rawValue)
			require.NoError(t, err)

			noOp, ok := got.(*NoOp)
			require.True(t, ok)
			assert.Equal(t, Name, noOp.GetName())
			assert.Equal(t, tt.override, noOp.GetOverride())
			assert.Equal(t, tt.rawValue, noOp.GetRawValue())
		})
	}
}

func TestNoOpValidation(t *testing.T) {
	tests := []struct {
		name      string
		noOp      *NoOp
		wantError bool
	}{
		{
			name:      "provider is required",
			noOp:      &NoOp{},
			wantError: true,
		},
		{
			name: "valid provider",
			noOp: func() *NoOp {
				got, err := New(false, false)
				require.NoError(t, err)

				typed, ok := got.(*NoOp)
				require.True(t, ok)

				return typed
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.Validate(tt.noOp)
			if tt.wantError {
				require.Error(t, err)
				assert.ErrorContains(t, err, "Provider")

				return
			}

			require.NoError(t, err)
		})
	}
}

//////
// Load behavior.
//////

func TestLoadReturnsEnvironment(t *testing.T) {
	const (
		key   = "CONFIGURER_NOOP_CONTRACT"
		value = "contract-value"
	)

	tests := []struct {
		name string
		opts []option.LoadKeyFunc
		want string
	}{
		{
			name: "identity",
			want: key,
		},
		{
			name: "option functions transform returned key",
			opts: []option.LoadKeyFunc{
				option.WithKeyPrefixer("PREFIX_"),
				option.WithKeySuffixer("_SUFFIX"),
			},
			want: "PREFIX_" + key + "_SUFFIX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(key, value)
			if tt.want != key {
				t.Setenv(tt.want, "")
			}

			noOp, err := New(true, false)
			require.NoError(t, err)

			got, err := noOp.Load(context.Background(), tt.opts...)
			require.NoError(t, err)
			assert.Equal(t, value, got[tt.want])
			assert.Equal(t, value, os.Getenv(tt.want))
		})
	}
}

func TestLoadRejectsInvalidTransformedKey(t *testing.T) {
	t.Setenv("CONFIGURER_NOOP_INVALID_KEY", "value")
	noOp, err := New(true, false)
	require.NoError(t, err)

	got, err := noOp.Load(
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
		wantError string
	}{
		{
			name:      "values are required",
			wantError: "values",
		},
		{
			name:   "accepts values as a no-op",
			values: map[string]interface{}{"KEY": "value"},
		},
		{
			name:   "accepts valid options as a no-op",
			values: map[string]interface{}{"KEY": "value"},
			opts: []option.WriteFunc{
				option.WithEnvironment("testing"),
				option.WithHTTPVerb("POST"),
				option.WithTarget("unused"),
				option.WithVariable(true),
			},
		},
		{
			name:   "option error is returned",
			values: map[string]interface{}{"KEY": "value"},
			opts: []option.WriteFunc{
				func(*option.Write) error { return sentinel },
			},
			wantError: sentinel.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noOp, err := New(false, false)
			require.NoError(t, err)

			var before map[string]interface{}
			if tt.values != nil {
				before = make(map[string]interface{}, len(tt.values))
				for key, value := range tt.values {
					before[key] = value
				}
			}

			err = noOp.Write(context.Background(), tt.values, tt.opts...)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantError)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, before, tt.values)
		})
	}
}
