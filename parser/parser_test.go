package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//////
// Factory.
//////

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "valid name",
			input: "env",
		},
		{
			name:    "empty name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "name too short",
			input:   "ab",
			wantErr: true,
		},
		{
			name:    "name too long",
			input:   strings.Repeat("a", 51),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.input, got.Name)
			assert.NotNil(t, got.Logger)
		})
	}
}
