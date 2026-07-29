package env

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//////
// Helpers.
//////

type errorReader struct {
	err error
}

func (r errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

//////
// Factory.
//////

func TestNew(t *testing.T) {
	got, err := New()

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Parser)
	assert.Equal(t, Name, got.Name)
}

//////
// Read.
//////

func TestENVRead(t *testing.T) {
	readErr := errors.New("read failed")

	tests := []struct {
		name    string
		reader  io.Reader
		want    map[string]any
		wantErr bool
	}{
		{
			name:   "valid content",
			reader: strings.NewReader("# comment\n\nPLAIN=value\nQUOTED=\"hello world\"\nWITH_EQUALS=a=b\n"),
			want: map[string]any{
				"PLAIN":       "value",
				"QUOTED":      "hello world",
				"WITH_EQUALS": "a=b",
			},
		},
		{
			name:    "malformed line",
			reader:  strings.NewReader("MISSING_SEPARATOR"),
			wantErr: true,
		},
		{
			name:   "empty input",
			reader: strings.NewReader(""),
			want:   map[string]any{},
		},
		{
			name:    "reader error",
			reader:  errorReader{err: readErr},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New()
			require.NoError(t, err)

			got, err := p.Read(context.Background(), tt.reader)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
