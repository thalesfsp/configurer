package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type C struct {
	Model string `json:"model"`
}

func TestReadConfigFile(t *testing.T) {
	// Create a temporary file in the OS temp directory.
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)

	// Load configuration, write default if needed.
	got, err := LoadConfiguration(tmpFile.Name(), "configurer", &C{Model: "default"})
	assert.NoError(t, err)
	assert.NotNil(t, got)

	// Read the temp file to assert the content.
	data, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)

	// Assert the content of the file.
	assert.Equal(t, true, strings.Contains(string(data), "model: default"))
}
