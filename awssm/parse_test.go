package awssm

import (
	"encoding/json"
	"testing"
)

// doubleEncode returns s wrapped as a JSON string literal, i.e. the form AWS
// Secrets Manager returns when a JSON payload was stored as a quoted string
// (double-encoded). For inner `{"K":"v"}` it yields `"{\"K\":\"v\"}"`.
func doubleEncode(t *testing.T, s string) string {
	t.Helper()

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("doubleEncode: %v", err)
	}

	return string(b)
}

func TestParseSecretData(t *testing.T) {
	object := `{"APP_KEY":"base64:abc+/=","DB_HOST":"prod.rds.amazonaws.com"}`

	tests := []struct {
		name       string
		input      string
		wantOK     bool
		wantKeys   map[string]string
		wantLength int
	}{
		{
			name:       "single-encoded JSON object",
			input:      object,
			wantOK:     true,
			wantKeys:   map[string]string{"APP_KEY": "base64:abc+/=", "DB_HOST": "prod.rds.amazonaws.com"},
			wantLength: 2,
		},
		{
			name:       "double-encoded object (JSON string containing JSON) - the regression",
			input:      doubleEncode(t, object),
			wantOK:     true,
			wantKeys:   map[string]string{"APP_KEY": "base64:abc+/=", "DB_HOST": "prod.rds.amazonaws.com"},
			wantLength: 2,
		},
		{
			name:       "triple-encoded object",
			input:      doubleEncode(t, doubleEncode(t, object)),
			wantOK:     true,
			wantKeys:   map[string]string{"APP_KEY": "base64:abc+/="},
			wantLength: 2,
		},
		{
			name:   "empty JSON object",
			input:  `{}`,
			wantOK: true,
		},
		{
			name:   "genuine plain text",
			input:  "hunter2",
			wantOK: false,
		},
		{
			name:   "JSON string that is not an object",
			input:  `"just a value"`,
			wantOK: false,
		},
		{
			name:   "JSON array is not a key/value object",
			input:  `["a","b"]`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSecretData(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseSecretData ok = %v, want %v", ok, tt.wantOK)
			}

			if tt.wantLength != 0 && len(got) != tt.wantLength {
				t.Errorf("len = %d, want %d", len(got), tt.wantLength)
			}

			for k, v := range tt.wantKeys {
				if fmt := toString(got[k]); fmt != v {
					t.Errorf("key %q = %q, want %q", k, fmt, v)
				}
			}
		})
	}
}

// TestParseSecretData_NegativeControl documents that the previous behaviour (a
// single json.Unmarshal into a map) fails on double-encoded secrets — which is
// exactly why they were mis-handled as a single plain-text value.
func TestParseSecretData_NegativeControl(t *testing.T) {
	double := doubleEncode(t, `{"APP_KEY":"v"}`)

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(double), &m); err == nil {
		t.Fatalf("expected plain Unmarshal to FAIL on double-encoded input, but it succeeded: %v", m)
	}

	if _, ok := parseSecretData(double); !ok {
		t.Fatalf("parseSecretData should recover the double-encoded object")
	}
}

func toString(v interface{}) string {
	s, _ := v.(string)

	return s
}
