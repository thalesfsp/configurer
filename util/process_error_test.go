package util

import "testing"

// TestSetDefault_InvalidValuePropagatesError is the bad-path coverage that was
// missing: an unparseable `default` tag must surface as an error instead of
// being silently swallowed by process().
func TestSetDefault_InvalidValueOnInt(t *testing.T) {
	type cfg struct {
		Port int `default:"not-a-number"`
	}

	if err := SetDefault(&cfg{}); err == nil {
		t.Fatal("expected error for invalid int default, got nil")
	}
}

// TestSetEnv_InvalidValuePropagatesError ensures env values that cannot be
// parsed into the target type also surface as errors.
func TestSetEnv_InvalidValueOnInt(t *testing.T) {
	type cfg struct {
		Port int `env:"CONFIGURER_TEST_PORT"`
	}

	t.Setenv("CONFIGURER_TEST_PORT", "not-a-number")

	if err := SetEnv(&cfg{}); err == nil {
		t.Fatal("expected error for invalid int env value, got nil")
	}
}
