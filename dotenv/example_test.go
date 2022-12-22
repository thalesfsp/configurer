package dotenv_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/thalesfsp/configurer/dotenv"
	"github.com/thalesfsp/configurer/util"
)

// ExampleNew creates a new dotenv instance and loads the .env file.
func ExampleNew() {
	// Instantiate the provider of choice, in this case dotenv.
	de, err := dotenv.New(false, "testing.env")
	if err != nil {
		log.Fatalln(err)
	}

	// Load will export to the environment all the variables.
	if _, err := de.Load(context.Background()); err != nil {
		log.Fatalln(err)
	}

	// We can check it here.
	fmt.Println(os.Getenv("TEST_KEY"))

	// Additionally, we can use the config into a struct.
	type Config struct {
		TestKey string `json:"testKey" env:"TEST_KEY"`
	}

	// ExportToStruct can be called from the provider...
	var c1 Config
	if err := de.ExportToStruct(&c1); err != nil {
		log.Fatalln(err)
	}

	// or from the `util` package.
	var c2 Config
	if err := util.Dump(&c2); err != nil {
		log.Fatalln(err)
	}

	// We can check it here.
	fmt.Println(c1.TestKey)
	fmt.Println(c2.TestKey)

	// Output:
	// TEST_VALUE
	// TEST_VALUE
	// TEST_VALUE
}
