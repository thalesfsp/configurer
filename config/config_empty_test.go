package config

import (
	"os"
	"path/filepath"
	"testing"
)

type sampleConfig struct {
	Model string `yaml:"model"`
}

// TestLoadConfiguration_EmptyFile covers the edge case of an existing but empty
// config file. The default must be written to disk and returned (this exercises
// the path that previously relied on variable shadowing).
func TestLoadConfiguration_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	def := &sampleConfig{Model: "default"}

	got, err := LoadConfiguration(path, "", def)
	if err != nil {
		t.Fatalf("LoadConfiguration returned error: %v", err)
	}

	if got.Model != "default" {
		t.Fatalf("got.Model = %q, want %q", got.Model, "default")
	}

	// The default must have been persisted to the previously empty file.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("expected default config to be written to the empty file")
	}
}

// TestLoadConfiguration_ExistingFile is the happy path: an existing populated
// file is read back into the struct.
func TestLoadConfiguration_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("model: custom\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadConfiguration(path, "", &sampleConfig{Model: "default"})
	if err != nil {
		t.Fatalf("LoadConfiguration returned error: %v", err)
	}

	if got.Model != "custom" {
		t.Fatalf("got.Model = %q, want %q", got.Model, "custom")
	}
}
