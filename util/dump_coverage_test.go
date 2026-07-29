package util

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//////
// Reflection-based setters.
//////

func TestSetDefaultReflectionBranches(t *testing.T) {
	type nested struct {
		Name   string          `default:"nested"`
		Values []int           `default:"1,2,3"`
		Flags  map[string]bool `default:"enabled:true,disabled:false"`
	}

	type config struct {
		Nested       nested
		NestedPtr    *nested
		NilNestedPtr *nested
		Count        *int   `default:"42"`
		Existing     string `default:"replacement"`
		Ignored      string `default:"-"`
		private      string `default:"hidden"`
	}

	got := &config{
		NestedPtr: &nested{},
		Existing:  "keep",
	}

	require.NoError(t, SetDefault(got))
	assert.Equal(t, nested{
		Name:   "nested",
		Values: []int{1, 2, 3},
		Flags:  map[string]bool{"enabled": true, "disabled": false},
	}, got.Nested)
	assert.Equal(t, &nested{
		Name:   "nested",
		Values: []int{1, 2, 3},
		Flags:  map[string]bool{"enabled": true, "disabled": false},
	}, got.NestedPtr)
	assert.Nil(t, got.NilNestedPtr)
	require.NotNil(t, got.Count)
	assert.Equal(t, 42, *got.Count)
	assert.Equal(t, "keep", got.Existing)
	assert.Empty(t, got.Ignored)
	assert.Empty(t, got.private)
}

func TestSetEnvReflectionBranches(t *testing.T) {
	t.Setenv("CONFIGURER_ZERO_CONTROL_CHAR", "clear")
	t.Setenv("CONFIGURER_TEST_ZERO_INT", "clear")
	t.Setenv("CONFIGURER_TEST_ZERO_UINT", "clear")
	t.Setenv("CONFIGURER_TEST_ZERO_FLOAT", "clear")
	t.Setenv("CONFIGURER_TEST_ZERO_BOOL", "clear")
	t.Setenv("CONFIGURER_TEST_ZERO_TIME", "clear")
	t.Setenv("CONFIGURER_TEST_ZERO_DURATION", "clear")
	t.Setenv("CONFIGURER_TEST_NESTED", "from-env")
	t.Setenv("CONFIGURER_TEST_SLICE", "4,5,6")
	t.Setenv(
		"CONFIGURER_TEST_MAP",
		"duration:1h,unsigned:18446744073709551615,date:2024-01-02,text:hello",
	)

	type nested struct {
		Value string `env:"CONFIGURER_TEST_NESTED"`
	}

	type config struct {
		Int      int           `env:"CONFIGURER_TEST_ZERO_INT"`
		Uint     uint          `env:"CONFIGURER_TEST_ZERO_UINT"`
		Float    float64       `env:"CONFIGURER_TEST_ZERO_FLOAT"`
		Bool     bool          `env:"CONFIGURER_TEST_ZERO_BOOL"`
		Time     time.Time     `env:"CONFIGURER_TEST_ZERO_TIME"`
		Duration time.Duration `env:"CONFIGURER_TEST_ZERO_DURATION"`
		Nested   nested
		Pointer  *nested
		Slice    []int                  `env:"CONFIGURER_TEST_SLICE"`
		Map      map[string]interface{} `env:"CONFIGURER_TEST_MAP"`
	}

	got := &config{
		Int:      1,
		Uint:     1,
		Float:    1,
		Bool:     true,
		Time:     time.Now(),
		Duration: time.Hour,
		Pointer:  &nested{},
	}

	require.NoError(t, SetEnv(got))
	assert.Zero(t, got.Int)
	assert.Zero(t, got.Uint)
	assert.Zero(t, got.Float)
	assert.False(t, got.Bool)
	assert.True(t, got.Time.IsZero())
	assert.Zero(t, got.Duration)
	assert.Equal(t, "from-env", got.Nested.Value)
	assert.Equal(t, "from-env", got.Pointer.Value)
	assert.Equal(t, []int{4, 5, 6}, got.Slice)
	assert.Equal(t, time.Hour, got.Map["duration"])
	assert.Equal(t, uint64(18446744073709551615), got.Map["unsigned"])
	assert.Equal(t, time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), got.Map["date"])
	assert.Equal(t, "hello", got.Map["text"])
}

func TestSetDefaultErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "invalid int",
			run: func() error {
				value := struct {
					Field int `default:"invalid"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid uint",
			run: func() error {
				value := struct {
					Field uint `default:"invalid"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid float",
			run: func() error {
				value := struct {
					Field float64 `default:"invalid"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid bool",
			run: func() error {
				value := struct {
					Field bool `default:"invalid"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid time",
			run: func() error {
				value := struct {
					Field time.Time `default:"not-a-time"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid duration",
			run: func() error {
				value := struct {
					Field time.Duration `default:"not-a-duration"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid slice element",
			run: func() error {
				value := struct {
					Field []int `default:"1,invalid"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "unsupported slice struct",
			run: func() error {
				value := struct {
					Field []struct{} `default:"value"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "unsupported slice kind",
			run: func() error {
				value := struct {
					Field []complex64 `default:"1"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid map pair",
			run: func() error {
				value := struct {
					Field map[string]string `default:"missing-separator"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid map key",
			run: func() error {
				value := struct {
					Field map[int]string `default:"invalid:value"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "invalid map value",
			run: func() error {
				value := struct {
					Field map[string]int `default:"key:invalid"`
				}{}

				return SetDefault(&value)
			},
		},
		{
			name: "unsupported kind",
			run: func() error {
				value := struct {
					Field chan int `default:"value"`
				}{}

				return SetDefault(&value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.run())
		})
	}
}

func TestSetIDErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "unsupported id type",
			run: func() error {
				value := struct {
					ID string `id:"snowflake"`
				}{}

				return SetID(&value)
			},
		},
		{
			name: "uuid cannot populate int",
			run: func() error {
				value := struct {
					ID int `id:"uuid"`
				}{}

				return SetID(&value)
			},
		},
		{
			name: "non-pointer input",
			run: func() error {
				return SetID(struct{}{})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.run())
		})
	}
}

//////
// Dump.
//////

func TestDumpErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "default error",
			run: func(_ *testing.T) error {
				value := struct {
					Port int `default:"invalid"`
				}{}

				return Dump(&value)
			},
		},
		{
			name: "environment error",
			run: func(t *testing.T) error {
				t.Helper()

				t.Setenv("CONFIGURER_TEST_DUMP_PORT", "invalid")
				value := struct {
					Port int `env:"CONFIGURER_TEST_DUMP_PORT"`
				}{}

				return Dump(&value)
			},
		},
		{
			name: "id error",
			run: func(_ *testing.T) error {
				value := struct {
					ID string `id:"invalid"`
				}{}

				return Dump(&value)
			},
		},
		{
			name: "validation error",
			run: func(_ *testing.T) error {
				value := struct {
					Name string `validate:"required"`
				}{}

				return Dump(&value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.run(t))
		})
	}
}

//////
// File dump helpers.
//////

func TestDumpToFile(t *testing.T) {
	tests := []struct {
		name    string
		dump    func(file *os.File) error
		contain string
	}{
		{
			name: "env",
			dump: func(file *os.File) error {
				return DumpToEnv(file, map[string]string{"KEY": "value"}, false)
			},
			contain: "KEY=value",
		},
		{
			name: "env raw",
			dump: func(file *os.File) error {
				return DumpToEnv(file, map[string]string{"KEY": "value"}, true)
			},
			contain: `KEY="value"`,
		},
		{
			name: "json",
			dump: func(file *os.File) error {
				return DumpToJSON(file, map[string]string{"KEY": "value"})
			},
			contain: `"KEY": "value"`,
		},
		{
			name: "yaml",
			dump: func(file *os.File) error {
				return DumpToYAML(file, map[string]string{"KEY": "value"})
			},
			contain: "KEY: value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "dump")
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, file.Close())
			})

			require.NoError(t, tt.dump(file))

			content, err := os.ReadFile(file.Name())
			require.NoError(t, err)
			assert.Contains(t, string(content), tt.contain)
		})
	}
}

func TestDumpToFileErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "env write",
			run: func(t *testing.T) error {
				t.Helper()

				return DumpToEnv(closedFile(t), map[string]string{"KEY": "value"}, false)
			},
		},
		{
			name: "env raw write",
			run: func(t *testing.T) error {
				t.Helper()

				return DumpToEnv(closedFile(t), map[string]string{"KEY": "value"}, true)
			},
		},
		{
			name: "env sync",
			run: func(t *testing.T) error {
				t.Helper()

				return withPipeWriter(t, func(file *os.File) error {
					return DumpToEnv(file, map[string]string{}, false)
				})
			},
		},
		{
			name: "json write",
			run: func(t *testing.T) error {
				t.Helper()

				return DumpToJSON(closedFile(t), map[string]string{"KEY": "value"})
			},
		},
		{
			name: "json sync",
			run: func(t *testing.T) error {
				t.Helper()

				return withPipeWriter(t, func(file *os.File) error {
					return DumpToJSON(file, map[string]string{"KEY": "value"})
				})
			},
		},
		{
			name: "yaml write",
			run: func(t *testing.T) error {
				t.Helper()

				return DumpToYAML(closedFile(t), map[string]string{"KEY": "value"})
			},
		},
		{
			name: "yaml sync",
			run: func(t *testing.T) error {
				t.Helper()

				return withPipeWriter(t, func(file *os.File) error {
					return DumpToYAML(file, map[string]string{"KEY": "value"})
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)

			require.Error(t, err)
			assert.True(t,
				strings.Contains(err.Error(), "write") || strings.Contains(err.Error(), "flush"),
				"unexpected error: %v",
				err,
			)
		})
	}
}

func closedFile(t *testing.T) *os.File {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "closed")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	return file
}

func withPipeWriter(t *testing.T, dump func(file *os.File) error) error {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reader.Close())
		require.NoError(t, writer.Close())
	})

	return dump(writer)
}
