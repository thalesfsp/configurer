//go:build unix

package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalesfsp/sypl/es/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/output"
	"github.com/thalesfsp/sypl/v2/processor"
)

const (
	concurrentRunnerHelperEnv = "CONFIGURER_CONCURRENT_RUNNER_HELPER"
	concurrentRunnerMarkerEnv = "CONFIGURER_CONCURRENT_RUNNER_MARKER"
	concurrentRunnerScriptEnv = "CONFIGURER_CONCURRENT_RUNNER_SCRIPT"
	elasticsearchHelperEnv    = "CONFIGURER_ELASTICSEARCH_HELPER"
)

//////
// Argument and structured-data helpers.
//////

func TestSplitCmdFromArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantCommand string
		wantArgs    []string
	}{
		{
			name:        "happy path command with arguments",
			args:        []string{"sh", "-c", "printf test"},
			wantCommand: "sh",
			wantArgs:    []string{"-c", "printf test"},
		},
		{
			name:        "edge case command without arguments",
			args:        []string{"/usr/bin/true"},
			wantCommand: "/usr/bin/true",
		},
		{
			name: "edge case empty input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, args := splitCmdFromArgs(tt.args)

			assert.Equal(t, tt.wantCommand, command)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestFlatMap(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  map[string]interface{}
	}{
		{
			name: "happy path nested maps and arrays",
			input: map[string]interface{}{
				"app": map[string]interface{}{
					"name": "configurer",
					"database": map[string]interface{}{
						"host": "localhost",
						"ports": []interface{}{
							5432,
							map[string]interface{}{"read": 5433},
							[]interface{}{true, map[string]interface{}{"deep": "value"}},
						},
					},
				},
				"enabled": true,
			},
			want: map[string]interface{}{
				"app.name":          "configurer",
				"app.database.host": "localhost",
				"app.database.ports": []interface{}{
					5432,
					map[string]interface{}{"read": 5433},
					[]interface{}{true, map[string]interface{}{"deep": "value"}},
				},
				"enabled": true,
			},
		},
		{
			name:  "edge case empty map",
			input: map[string]interface{}{},
			want:  map[string]interface{}{},
		},
		{
			name: "edge case nil and empty array values",
			input: map[string]interface{}{
				"empty": []interface{}{},
				"nil":   nil,
			},
			want: map[string]interface{}{
				"empty": []interface{}{},
				"nil":   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, flatMap(tt.input))
		})
	}
}

func TestParseArray(t *testing.T) {
	tests := []struct {
		name  string
		input []interface{}
		want  []interface{}
	}{
		{
			name: "happy path recursively flattens map elements",
			input: []interface{}{
				map[string]interface{}{
					"outer": map[string]interface{}{"inner": "value"},
				},
				[]interface{}{
					map[string]interface{}{
						"another": map[string]interface{}{"leaf": 42},
					},
				},
			},
			want: []interface{}{
				map[string]interface{}{"outer.inner": "value"},
				[]interface{}{
					map[string]interface{}{"another.leaf": 42},
				},
			},
		},
		{
			name:  "edge case empty array",
			input: []interface{}{},
			want:  []interface{}{},
		},
		{
			name:  "edge case primitive and nil values remain unchanged",
			input: []interface{}{"value", 7, true, nil},
			want:  []interface{}{"value", 7, true, nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseArray(tt.input)

			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCommandArgsJSON(t *testing.T) {
	tests := []struct {
		name  string
		input CommandArgs
		want  string
	}{
		{
			name:  "happy path command and arguments",
			input: CommandArgs{Command: "echo", Args: []string{"hello", "world"}},
			want:  `{"args":["hello","world"],"command":"echo"}`,
		},
		{
			name:  "edge case zero value",
			input: CommandArgs{},
			want:  `{"args":null,"command":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(data))

			var decoded CommandArgs
			require.NoError(t, json.Unmarshal(data, &decoded))
			assert.Equal(t, tt.input, decoded)
		})
	}
}

//////
// Text loading and child execution.
//////

func TestLoadFromText(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		data           string
		key            string
		wantValue      string
		existingValue  string
		shouldOverride bool
		rawValue       bool
	}{
		{
			name:      "happy path env",
			format:    "env",
			data:      "CONFIGURER_TEXT_ENV=value\n",
			key:       "CONFIGURER_TEXT_ENV",
			wantValue: "value",
		},
		{
			name:      "happy path json numeric value",
			format:    "json",
			data:      `{"CONFIGURER_TEXT_JSON":42}`,
			key:       "CONFIGURER_TEXT_JSON",
			wantValue: "42",
		},
		{
			name:          "edge case existing value is preserved without override",
			format:        "env",
			data:          "CONFIGURER_TEXT_COLLISION=loaded\n",
			key:           "CONFIGURER_TEXT_COLLISION",
			existingValue: "existing",
			wantValue:     "existing",
		},
		{
			name:           "edge case raw and override options retained by provider",
			format:         "yaml",
			data:           "CONFIGURER_TEXT_YAML: spaced value\n",
			key:            "CONFIGURER_TEXT_YAML",
			wantValue:      `"spaced value"`,
			shouldOverride: true,
			rawValue:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.existingValue)

			loadedProvider, err := LoadFromText(
				tt.shouldOverride,
				tt.rawValue,
				tt.format,
				tt.data,
			)

			require.NoError(t, err)
			require.NotNil(t, loadedProvider)
			assert.Equal(t, tt.shouldOverride, loadedProvider.GetOverride())
			assert.Equal(t, tt.rawValue, loadedProvider.GetRawValue())
			assert.Equal(t, tt.wantValue, os.Getenv(tt.key))
		})
	}
}

func TestLoadFromTextErrors(t *testing.T) {
	tests := []struct {
		name   string
		format string
		data   string
	}{
		{
			name:   "bad path unsupported format",
			format: "xml",
			data:   "<value />",
		},
		{
			name:   "bad path malformed json",
			format: "json",
			data:   `{"broken":`,
		},
		{
			name:   "bad path malformed yaml",
			format: "yaml",
			data:   "key: [",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadedProvider, err := LoadFromText(false, false, tt.format, tt.data)

			require.Error(t, err)
			assert.Nil(t, loadedProvider)
		})
	}
}

func TestRunCommand(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		args           []string
		combinedOutput bool
		wantExitCode   int
		wantOutput     []string
	}{
		{
			name:         "happy path trivial child",
			command:      "/usr/bin/true",
			wantExitCode: 0,
		},
		{
			name:           "happy path combined stdout and stderr",
			command:        "/bin/sh",
			args:           []string{"-c", `printf "child-stdout\n"; printf "child-stderr\n" >&2`},
			combinedOutput: true,
			wantExitCode:   0,
			wantOutput:     []string{"stdout child-stdout", "stderr child-stderr"},
		},
		{
			name:         "bad path child exit code is propagated",
			command:      "/bin/sh",
			args:         []string{"-c", "exit 7"},
			wantExitCode: 7,
		},
		{
			name:         "bad path missing executable",
			command:      filepath.Join(t.TempDir(), "missing-command"),
			wantExitCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, exitCode := captureStdout(t, func() int {
				return runCommand(nil, tt.command, tt.args, tt.combinedOutput)
			})

			assert.Equal(t, tt.wantExitCode, exitCode)
			for _, want := range tt.wantOutput {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestRunCommandReceivesLoadedEnvironment(t *testing.T) {
	const key = "CONFIGURER_RUN_COMMAND_LOADED"

	t.Setenv(key, "")
	loadedProvider, err := LoadFromText(true, false, "env", key+"=forwarded\n")
	require.NoError(t, err)

	output, exitCode := captureStdout(t, func() int {
		return runCommand(
			loadedProvider,
			"/bin/sh",
			[]string{"-c", `printf "%s" "$CONFIGURER_RUN_COMMAND_LOADED"`},
			false,
		)
	})

	assert.Zero(t, exitCode)
	assert.Equal(t, "forwarded", output)
}

func TestRunCommandElasticsearchConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		mode         string
		wantExitCode int
		wantOutput   string
	}{
		{
			name:         "bad path missing settings",
			mode:         "missing-settings",
			wantExitCode: 1,
			wantOutput:   "missing log settings",
		},
		{
			name:         "bad path malformed settings",
			mode:         "malformed-settings",
			wantExitCode: 1,
			wantOutput:   "failed to parse log settings",
		},
		{
			name:         "bad path missing index",
			mode:         "missing-index",
			wantExitCode: 1,
			wantOutput:   "index is missing",
		},
		{
			name: "happy path default address",
			mode: "default-address",
		},
		{
			name: "happy path explicit address and combined mode",
			mode: "explicit-address-combined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command(
				os.Args[0],
				"-test.run=^TestRunCommandElasticsearchHelper$",
			)
			command.Env = append(os.Environ(), elasticsearchHelperEnv+"="+tt.mode)

			output, err := command.CombinedOutput()

			assertProcessExitCode(t, tt.wantExitCode, err, string(output))
			assert.Contains(t, string(output), tt.wantOutput)
		})
	}
}

func TestRunCommandElasticsearchHelper(t *testing.T) {
	mode := os.Getenv(elasticsearchHelperEnv)
	if mode == "" {
		return
	}

	logOutputs = []string{"stdout", "elasticsearch"}
	flushInterval = time.Millisecond

	combinedOutput := false
	switch mode {
	case "missing-settings":
		logSettings = ""
	case "malformed-settings":
		logSettings = "{"
	case "missing-index":
		logSettings = `{"addresses":["http://127.0.0.1:1"]}`
	case "default-address":
		logSettings = `{"index":"configurer-test"}`
		installFakeElasticsearchOutput()
	case "explicit-address-combined":
		logSettings = `{"index":"configurer-test","addresses":["http://127.0.0.1:1"]}`
		combinedOutput = true
		installFakeElasticsearchOutput()
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}

	os.Exit(runCommand(nil, "/usr/bin/true", nil, combinedOutput))
}

func installFakeElasticsearchOutput() {
	newElasticsearchOutput = func(
		_ es.DynamicIndexFunc,
		_ es.Config,
		maxLevel level.Level,
		processors ...processor.IProcessor,
	) output.IOutput {
		return output.Console(maxLevel, processors...)
	}
}

func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	originalStdout := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	var output bytes.Buffer
	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&output, reader)
		copyDone <- copyErr
	}()

	result := fn()

	require.NoError(t, writer.Close())
	os.Stdout = originalStdout
	require.NoError(t, <-copyDone)
	require.NoError(t, reader.Close())

	return output.String(), result
}

//////
// ConcurrentRunner subprocess coverage.
//////

func TestConcurrentRunner(t *testing.T) {
	tempDir := t.TempDir()
	markerFile := filepath.Join(tempDir, "commands.log")
	scriptFile := filepath.Join(tempDir, "command.sh")
	require.NoError(t, os.WriteFile(scriptFile, []byte(`#!/bin/sh
printf '%s\n' "$1" >> "$CONFIGURER_CONCURRENT_RUNNER_MARKER"
if [ "$1" = "fail7" ]; then
	exit 7
fi
`), 0o755))

	tests := []struct {
		name         string
		mode         string
		wantExitCode int
		wantMarkers  []string
	}{
		{
			name:        "happy path direct command",
			mode:        "direct-success",
			wantMarkers: []string{"direct"},
		},
		{
			name:         "bad path direct command propagates exit code",
			mode:         "direct-failure",
			wantExitCode: 7,
			wantMarkers:  []string{"fail7"},
		},
		{
			name: "edge case no command exits successfully",
			mode: "no-command",
		},
		{
			name:        "happy path multiple concurrent commands",
			mode:        "multiple-concurrent",
			wantMarkers: []string{"first", "second"},
		},
		{
			name:         "bad path concurrent command failure",
			mode:         "multiple-concurrent-failure",
			wantExitCode: 1,
			wantMarkers:  []string{"fail7", "first"},
		},
		{
			name:        "happy path multiple sequential commands",
			mode:        "multiple-sequential",
			wantMarkers: []string{"first", "second"},
		},
		{
			name:         "bad path sequential command propagates exit code",
			mode:         "multiple-sequential-failure",
			wantExitCode: 7,
			wantMarkers:  []string{"fail7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(markerFile, nil, 0o600))

			command := exec.Command(
				os.Args[0],
				"-test.run=^TestConcurrentRunnerHelper$",
			)
			command.Env = append(
				os.Environ(),
				concurrentRunnerHelperEnv+"="+tt.mode,
				concurrentRunnerMarkerEnv+"="+markerFile,
				concurrentRunnerScriptEnv+"="+scriptFile,
			)

			output, err := command.CombinedOutput()
			assertProcessExitCode(t, tt.wantExitCode, err, string(output))

			data, readErr := os.ReadFile(markerFile)
			require.NoError(t, readErr)

			var gotMarkers []string
			if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
				gotMarkers = strings.Split(trimmed, "\n")
				sort.Strings(gotMarkers)
			}

			wantMarkers := append([]string(nil), tt.wantMarkers...)
			sort.Strings(wantMarkers)
			assert.Equal(t, wantMarkers, gotMarkers, "subprocess output:\n%s", output)
		})
	}
}

func TestConcurrentRunnerHelper(t *testing.T) {
	mode := os.Getenv(concurrentRunnerHelperEnv)
	if mode == "" {
		return
	}

	flushInterval = time.Millisecond
	sequentialDelay = time.Millisecond
	logOutputs = nil
	logSettings = ""

	script := os.Getenv(concurrentRunnerScriptEnv)

	switch mode {
	case "direct-success":
		ConcurrentRunner(nil, nil, []string{script, "direct"})
	case "direct-failure":
		ConcurrentRunner(nil, nil, []string{script, "fail7"})
	case "no-command":
		ConcurrentRunner(nil, nil, nil)
	case "multiple-concurrent":
		execMode = "concurrent"
		ConcurrentRunner(nil, []string{script + " first", script + " second"}, nil)
	case "multiple-concurrent-failure":
		execMode = "concurrent"
		ConcurrentRunner(nil, []string{script + " first", script + " fail7"}, nil)
	case "multiple-sequential":
		execMode = "sequential"
		ConcurrentRunner(nil, []string{script + " first", script + " second"}, nil)
	case "multiple-sequential-failure":
		execMode = "sequential"
		ConcurrentRunner(nil, []string{script + " fail7", script + " never"}, nil)
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
}

func assertProcessExitCode(t *testing.T, want int, err error, output string) {
	t.Helper()

	if want == 0 {
		require.NoError(t, err, "subprocess output:\n%s", output)

		return
	}

	var exitError *exec.ExitError
	require.ErrorAs(t, err, &exitError, "subprocess output:\n%s", output)
	assert.Equal(t, want, exitError.ExitCode(), "subprocess output:\n%s", output)
}
