package yaml

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/parser"
)

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

func TestNewBaseParserError(t *testing.T) {
	wantErr := errors.New("base parser failed")
	original := newBaseParser
	t.Cleanup(func() {
		newBaseParser = original
	})
	newBaseParser = func(_ string) (*parser.Parser, error) {
		return nil, wantErr
	}

	got, err := New()

	require.ErrorIs(t, err, wantErr)
	assert.Nil(t, got)
}

//////
// Read.
//////

func TestYAMLRead(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]any
		wantErr bool
	}{
		{
			name:    "valid content",
			content: "name: configurer\nenabled: true\ncount: 2\n",
			want: map[string]any{
				"name":    "configurer",
				"enabled": true,
				"count":   2,
			},
		},
		{
			name:    "malformed input",
			content: "name: [unterminated",
			wantErr: true,
		},
		{
			name:    "empty input",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New()
			require.NoError(t, err)

			got, err := p.Read(context.Background(), strings.NewReader(tt.content))

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
