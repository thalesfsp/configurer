package noop

import (
	"context"
	"os"
	"testing"

	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
)

func TestNoOp_Load(t *testing.T) {
	type fields struct {
		Provider *provider.Provider
	}
	tests := []struct {
		fields      fields
		name        string
		opts        []option.LoadKeyFunc
		override    bool
		wantErr     bool
		wantKey     string
		wantLoadErr bool
	}{
		{
			name:    "should work",
			wantKey: "TEST_KEY",
			wantErr: false,
		},
		{
			name: "should work with options",
			opts: []option.LoadKeyFunc{
				option.WithKeyCaser("upper"),
				option.WithKeyPrefixer("TESTING_DOTENV_"),
			},
			wantKey: "TESTING_DOTENV_TEST_KEY",
			wantErr: false,
		},
		{
			name: "should work with options - replacer",
			opts: []option.LoadKeyFunc{
				option.WithKeyReplacer(func(key string) string {
					return "TESTING123_" + key
				}),
				option.WithKeyCaser("lower"),
			},
			wantKey: "testing123_test_key",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_KEY", "TEST_VALUE")
			defer os.Unsetenv(tt.wantKey)

			d, err := New(tt.override)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if d != nil {
				_, err := d.Load(context.Background(), tt.opts...)
				if (err != nil) != tt.wantLoadErr {
					t.Errorf("DotEnv.Load() error = %v, wantLoadErr %v", err, tt.wantLoadErr)
					return
				}

				if err == nil {
					if os.Getenv(tt.wantKey) != "TEST_VALUE" {
						t.Log("HERE", os.Environ())
						t.Errorf("Loaded config error = %v, wantErr %v", os.Getenv(tt.wantKey), "TEST_VALUE")
						return
					}
				}
			}
		})
	}
}
