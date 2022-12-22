package noop_test

import (
	"context"
	"fmt"
	"os"

	"github.com/thalesfsp/configurer/noop"
	"github.com/thalesfsp/configurer/util"
)

// ExampleNew creates a new noop instance and loads from it.
func ExampleNew() {
	const key = "TEST_KEY"

	os.Setenv(key, "TEST_VALUE")
	defer os.Unsetenv(key)

	// Instantiate the provider of choice, in this case noop.
	de, err := noop.New(false)
	if err != nil {
		panic(err)
	}

	// Load will export to the environment all the variables.
	if _, err := de.Load(context.Background()); err != nil {
		panic(err)
	}

	// We can check it here.
	fmt.Println(os.Getenv(key))

	// Additionally, we can use the config into a struct.
	type Config struct {
		TestKey string `json:"testKey" env:"TEST_KEY"`
	}

	// ExportToStruct can be called from the provider...
	var c1 Config
	if err := de.ExportToStruct(&c1); err != nil {
		panic(err)
	}

	// or from the `util` package.
	var c2 Config
	if err := util.Dump(&c2); err != nil {
		panic(err)
	}

	// We can check it here.
	fmt.Println(c1.TestKey)
	fmt.Println(c2.TestKey)

	// Output:
	// TEST_VALUE
	// TEST_VALUE
	// TEST_VALUE
}
