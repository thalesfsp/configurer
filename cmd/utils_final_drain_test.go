//go:build unix

package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/sypl/es/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
	"github.com/thalesfsp/sypl/v2/processor"
)

func TestRunCommandDrainsESBuffersOnNormalExit(t *testing.T) {
	originalFlushInterval := flushInterval
	originalLogOutputs := logOutputs
	originalLogSettings := logSettings
	originalNewElasticsearchOutput := newElasticsearchOutput

	t.Cleanup(func() {
		flushInterval = originalFlushInterval
		logOutputs = originalLogOutputs
		logSettings = originalLogSettings
		newElasticsearchOutput = originalNewElasticsearchOutput
	})

	logOutputs = []string{"elasticsearch"}
	logSettings = `{"index":"configurer-test","addresses":["http://127.0.0.1:1"]}`
	flushInterval = time.Hour

	tests := []struct {
		name         string
		script       string
		wantExitCode int
		wantLogged   []string
	}{
		{
			name:         "happy path drains complete lines after successful exit",
			script:       `printf "success-stdout\n"; printf "success-stderr\n" >&2`,
			wantLogged:   []string{"success-stdout", "success-stderr"},
			wantExitCode: 0,
		},
		{
			name:         "bad path drains buffers after non-zero exit",
			script:       `printf "failure-stdout\n"; printf "failure-stderr\n" >&2; exit 7`,
			wantLogged:   []string{"failure-stdout", "failure-stderr"},
			wantExitCode: 7,
		},
		{
			name:         "edge case drains partial lines without trailing newline",
			script:       `printf "partial-stdout"; printf "partial-stderr" >&2`,
			wantLogged:   []string{"partial-stdout", "partial-stderr"},
			wantExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logged := new(syncBuffer)
			newElasticsearchOutput = func(
				_ es.DynamicIndexFunc,
				_ es.Config,
				maxLevel level.Level,
				processors ...processor.IProcessor,
			) output.IOutput {
				return output.New("final-drain-test", maxLevel, logged, processors...)
			}

			exitCode := runCommand(nil, "/bin/sh", []string{"-c", tt.script}, false)
			loggedOutput := logged.readRemaining()

			require.Equal(t, tt.wantExitCode, exitCode)
			for _, want := range tt.wantLogged {
				assert.Contains(t, loggedOutput, want)
			}
		})
	}
}
