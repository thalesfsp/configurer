package util

import (
	"context"
	"reflect"
	"testing"
)

// TestParseContent_Formats is the happy-path coverage that was missing and that
// would have caught the YAML-parsed-with-env-parser bug. It exercises every
// format ParseContent claims to support.
func TestParseContent_Formats(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		format string
		data   string
		want   map[string]any
	}{
		{
			name:   "env",
			format: "env",
			data:   "FOO=bar\nBAZ=qux\n",
			want:   map[string]any{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:   "json",
			format: "json",
			data:   `{"FOO":"bar","BAZ":"qux"}`,
			want:   map[string]any{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:   "yaml",
			format: "yaml",
			data:   "FOO: bar\nBAZ: qux\n",
			want:   map[string]any{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:   "yml alias",
			format: "yml",
			data:   "FOO: bar\nBAZ: qux\n",
			want:   map[string]any{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:   "toml",
			format: "toml",
			data:   "FOO = \"bar\"\nBAZ = \"qux\"\n",
			want:   map[string]any{"FOO": "bar", "BAZ": "qux"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFromText(ctx, tc.format, tc.data)
			if err != nil {
				t.Fatalf("ParseFromText(%q) returned error: %v", tc.format, err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseFromText(%q) = %#v, want %#v", tc.format, got, tc.want)
			}
		})
	}
}

// TestParseContent_YAMLNested guards the regression specifically: nested YAML
// (impossible to express in .env) must parse via the real YAML parser.
func TestParseContent_YAMLNested(t *testing.T) {
	got, err := ParseFromText(context.Background(), "yaml", "db:\n  host: localhost\n  port: 5432\n")
	if err != nil {
		t.Fatalf("nested YAML returned error: %v", err)
	}

	db, ok := got["db"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map for key db, got %#v", got["db"])
	}

	if db["host"] != "localhost" {
		t.Fatalf("db.host = %v, want localhost", db["host"])
	}
}

// TestParseContent_BadFormat is the bad-path coverage.
func TestParseContent_BadFormat(t *testing.T) {
	if _, err := ParseFromText(context.Background(), "xml", "irrelevant"); err == nil {
		t.Fatal("expected an error for unsupported format, got nil")
	}
}
