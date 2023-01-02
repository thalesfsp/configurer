package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestDumpToEnv(t *testing.T) {
	fileEnv, err := os.CreateTemp("", "test-*.env")
	if err != nil {
		t.Fatal(err)
	}

	defer fileEnv.Close()

	fileJSON, err := os.CreateTemp("", "test-*.json")
	if err != nil {
		t.Fatal(err)
	}

	defer fileJSON.Close()

	fileYML, err := os.CreateTemp("", "test-*.yml")
	if err != nil {
		t.Fatal(err)
	}

	defer fileYML.Close()

	if err := DumpToFile(fileEnv, map[string]string{
		"K1": "V1",
		"K2": "V2",
	}); err != nil {
		t.Fatal(err)
	}

	//////
	// ENV
	//////

	// Read and print the contents of the file.
	dataEnv, err := os.ReadFile(fileEnv.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(dataEnv), "K1=V1") {
		t.Fatal("expected file to contain K1=V1")
	}

	if !strings.Contains(string(dataEnv), "K2=V2") {
		t.Fatal("expected file to contain K2=V2")
	}

	//////
	// JSON
	//////

	if err := DumpToFile(fileJSON, map[string]string{
		"K1": "V1",
		"K2": "V2",
	}); err != nil {
		t.Fatal(err)
	}

	// Read and print the contents of the file.
	dataJSON, err := os.ReadFile(fileJSON.Name())
	if err != nil {
		t.Fatal(err)
	}

	dataJSONasString := string(dataJSON)

	if !strings.Contains(dataJSONasString, `"K1": "V1"`) {
		t.Fatal("expected file to contain K1: V1", dataJSONasString)
	}

	if !strings.Contains(dataJSONasString, `"K2": "V2"`) {
		t.Fatal("expected file to contain K2: V2", dataJSONasString)
	}

	//////
	// YML
	//////

	if err := DumpToFile(fileYML, map[string]string{
		"K1": "V1",
		"K2": "V2",
	}); err != nil {
		t.Fatal(err)
	}

	// Read and print the contents of the file.
	dataYML, err := os.ReadFile(fileYML.Name())
	if err != nil {
		t.Fatal(err)
	}

	dataYMLasString := string(dataYML)

	if !strings.Contains(dataYMLasString, `K1: V1`) {
		t.Fatal("expected file to contain K1: V1", dataYMLasString)
	}

	if !strings.Contains(dataYMLasString, `K2: V2`) {
		t.Fatal("expected file to contain K2: V2", dataYMLasString)
	}
}
