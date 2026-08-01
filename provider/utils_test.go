package provider

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/configurer/internal/logging"
	"github.com/thalesfsp/configurer/internal/testenv"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
	"github.com/thalesfsp/sypl/v2/shared"
)

//////
// Helpers.
//////

// newLoggerWithMaxLevels builds a logger with one discard output per level.
func newLoggerWithMaxLevels(levels ...level.Level) *sypl.Sypl {
	l := sypl.New("test")

	for i, lvl := range levels {
		l.AddOutputs(output.New("out-"+string(rune('a'+i)), lvl, io.Discard))
	}

	return l
}

type exportTestProvider struct {
	*Provider
}

func (p *exportTestProvider) Load(
	_ context.Context,
	_ ...option.LoadKeyFunc,
) (map[string]string, error) {
	return nil, ErrNotSupported
}

func (p *exportTestProvider) Write(
	_ context.Context,
	_ map[string]interface{},
	_ ...option.WriteFunc,
) error {
	return ErrNotSupported
}

//////
// Tests.
//////

// TestAnyMaxLevel proves the anyMaxLevel helper preserves the removed v1
// Sypl.AnyMaxLevel semantics used by the ExportToEnvVar guard: true when
// some output's maxLevel EQUALS the target (or SYPL_LEVEL names it), false
// otherwise.
func TestAnyMaxLevel(t *testing.T) {
	tests := []struct {
		name        string
		outputCaps  []level.Level
		envLevel    string
		wantDebug   bool
		wantTrace   bool
		description string
	}{
		{
			name:        "output capped at Debug",
			outputCaps:  []level.Level{level.Debug},
			wantDebug:   true,
			wantTrace:   false,
			description: "guard takes the Debug branch, as in v1",
		},
		{
			name:        "output capped at Trace",
			outputCaps:  []level.Level{level.Trace},
			wantDebug:   false,
			wantTrace:   true,
			description: "guard takes the Trace branch (logs the value), as in v1",
		},
		{
			name:        "output capped at Info",
			outputCaps:  []level.Level{level.Info},
			wantDebug:   false,
			wantTrace:   false,
			description: "neither branch fires, as in v1",
		},
		{
			name:        "no outputs",
			outputCaps:  nil,
			wantDebug:   false,
			wantTrace:   false,
			description: "neither branch fires, as in v1",
		},
		{
			name:        "mixed outputs Info+Debug",
			outputCaps:  []level.Level{level.Info, level.Debug},
			wantDebug:   true,
			wantTrace:   false,
			description: "any single matching output is enough, as in v1",
		},
		{
			name:        "SYPL_LEVEL env var names debug",
			outputCaps:  []level.Level{level.Info},
			envLevel:    level.Debug.String(),
			wantDebug:   true,
			wantTrace:   false,
			description: "v1 env-var fallback preserved",
		},
		{
			name:        "SYPL_LEVEL env var names trace",
			outputCaps:  []level.Level{level.Info},
			envLevel:    level.Trace.String(),
			wantDebug:   false,
			wantTrace:   true,
			description: "v1 env-var fallback preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv also isolates: unset means empty for this subtest.
			t.Setenv(shared.LevelEnvVar, tt.envLevel)

			l := newLoggerWithMaxLevels(tt.outputCaps...)

			if got := anyMaxLevel(l, level.Debug); got != tt.wantDebug {
				t.Errorf(
					"anyMaxLevel(l, Debug) = %v, want %v (%s)",
					got, tt.wantDebug, tt.description,
				)
			}

			if got := anyMaxLevel(l, level.Trace); got != tt.wantTrace {
				t.Errorf(
					"anyMaxLevel(l, Trace) = %v, want %v (%s)",
					got, tt.wantTrace, tt.description,
				)
			}
		})
	}
}

func TestExportToEnvVar(t *testing.T) {
	const key = "CONFIGURER_PROVIDER_EXPORT_TEST"

	// initialPresent distinguishes "variable absent from the environment" from
	// "variable present and set to the empty string". Both used to be spelled
	// initialValue: "", which is why the clobber went unnoticed.
	tests := []struct {
		name           string
		key            string
		initialValue   string
		initialPresent bool
		value          interface{}
		override       bool
		rawValue       bool
		logLevels      []level.Level
		want           string
		wantErr        bool
	}{
		{
			name:  "exports new string",
			key:   key,
			value: "loaded",
			want:  "loaded",
		},
		{
			name:           "existing environment value wins",
			key:            key,
			initialValue:   "existing",
			initialPresent: true,
			value:          "loaded",
			want:           "existing",
		},
		{
			name:           "override replaces existing environment value",
			key:            key,
			initialValue:   "existing",
			initialPresent: true,
			value:          "loaded",
			override:       true,
			want:           "loaded",
		},
		{
			name:  "empty incoming value remains empty",
			key:   key,
			value: "",
			want:  "",
		},
		{
			// Regression: a variable that is SET to the empty string is still
			// set, so it must win when override is off. The previous suite
			// asserted the opposite ("empty existing value does not block
			// export"), encoding the very bug it should have caught.
			name:           "present but empty existing value is preserved",
			key:            key,
			initialValue:   "",
			initialPresent: true,
			value:          "loaded",
			want:           "",
		},
		{
			name:           "override replaces present but empty existing value",
			key:            key,
			initialValue:   "",
			initialPresent: true,
			value:          "loaded",
			override:       true,
			want:           "loaded",
		},
		{
			name:  "special characters and multiline remain unquoted",
			key:   key,
			value: "pa$$ word=\"quoted\"\nnext=line",
			want:  "pa$$ word=\"quoted\"\nnext=line",
		},
		{
			name:     "raw string preserves Go quoting and escapes",
			key:      key,
			value:    "pa$$ word=\"quoted\"\nnext=line",
			rawValue: true,
			want:     "\"pa$$ word=\\\"quoted\\\"\\nnext=line\"",
		},
		{
			// The preserved environment value is returned byte for byte: raw
			// formatting only ever applies to the provider-sourced value.
			name:           "raw formatting is not applied to a preserved value",
			key:            key,
			initialValue:   "existing",
			initialPresent: true,
			value:          "loaded",
			rawValue:       true,
			want:           "existing",
		},
		{
			name:      "debug logging branch",
			key:       key,
			value:     42,
			logLevels: []level.Level{level.Debug},
			want:      "42",
		},
		{
			name:      "trace logging branch",
			key:       key,
			value:     true,
			logLevels: []level.Level{level.Trace},
			want:      "true",
		},
		{
			name:    "invalid environment key",
			key:     "INVALID=KEY",
			value:   "loaded",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(shared.LevelEnvVar, "")

			if tt.key != "INVALID=KEY" {
				testenv.SetPresence(t, tt.key, tt.initialValue, tt.initialPresent)
			}

			p := &exportTestProvider{
				Provider: &Provider{
					Logger: &logging.Logger{
						Sypl: newLoggerWithMaxLevels(tt.logLevels...),
					},
					Name:     "test",
					Override: tt.override,
					RawValue: tt.rawValue,
				},
			}

			got, err := ExportToEnvVar(p, tt.key, tt.value)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			testenv.RequireSet(t, tt.key, tt.want)
		})
	}
}
