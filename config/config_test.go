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
	// tmpFile, err := os.CreateTemp("", "config-*.yaml")
	// assert.NoError(t, err)

	filePath := "/var/folders/3n/x9cr9_cd6yj7flykwvl452p40000gn/T/config-829069182.yaml"

	t.Log("File path:", filePath)

	// Load configuration, write default if needed.
	got, err := LoadConfiguration(filePath, "configurer", &C{Model: "default"})
	assert.NoError(t, err)
	assert.NotNil(t, got)

	// Read the temp file to assert the content.
	data, err := os.ReadFile(filePath)
	assert.NoError(t, err)

	// Assert the content of the file.
	assert.Equal(t, true, strings.Contains(string(data), "model: default"))
}
