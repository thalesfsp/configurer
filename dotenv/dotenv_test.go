// Package dotenv provides a `.env` provider.
package dotenv

import (
	"context"
	"os"
	"testing"

	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
)

func TestDotEnv_Load(t *testing.T) {
	type fields struct {
		Provider  *provider.Provider
		FilePaths []string
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
			name: "should fail - no file paths",
			fields: fields{
				FilePaths: []string{},
			},
			wantKey: "TEST_KEY",
			wantErr: true,
		},
		{
			name: "should fail - invalid file paths",
			fields: fields{
				FilePaths: []string{"/asd/qwe/ert.env"},
			},
			wantKey:     "TEST_KEY",
			wantErr:     false,
			wantLoadErr: true,
		},
		{
			name: "should work",
			fields: fields{
				FilePaths: []string{"testing.env"},
			},
			wantKey: "TEST_KEY",
			wantErr: false,
		},
		{
			name: "should work with options",
			fields: fields{
				FilePaths: []string{"testing.env"},
			},
			opts: []option.LoadKeyFunc{
				option.WithKeyCaser("upper"),
				option.WithKeyPrefixer("TESTING_DOTENV_"),
			},
			wantKey: "TESTING_DOTENV_TEST_KEY",
			wantErr: false,
		},
		{
			name: "should work with options - replacer",
			fields: fields{
				FilePaths: []string{"testing.env"},
			},
			opts: []option.LoadKeyFunc{
				option.WithKeyReplacer(func(key string) string {
					return "TESTING123_" + key
				}),
				option.WithKeyCaser(option.Lower),
			},
			wantKey: "testing123_test_key",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Unsetenv(tt.wantKey)

			d, err := New(tt.override, tt.fields.FilePaths...)
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
						t.Log(os.Environ())
						t.Errorf("Loaded config error = %v, wantErr %v", os.Getenv(tt.wantKey), "TEST_VALUE")
						return
					}
				}
			}
		})
	}
}
