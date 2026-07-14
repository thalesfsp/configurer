package provider

import (
	"io"
	"testing"

	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
	"github.com/thalesfsp/sypl/v2/shared"
)

// newLoggerWithMaxLevels builds a logger with one discard output per level.
func newLoggerWithMaxLevels(levels ...level.Level) *sypl.Sypl {
	l := sypl.New("test")

	for i, lvl := range levels {
		l.AddOutputs(output.New("out-"+string(rune('a'+i)), lvl, io.Discard))
	}

	return l
}

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
