package e2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "configurer-e2e")
	if err != nil {
		panic(err)
	}

	binPath = filepath.Join(dir, "configurer")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = mustRepoRoot()
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(dir)

		panic(string(out))
	}

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}

func mustRepoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(out))
}

func run(t *testing.T, env []string, args ...string) (string, string, int) {
	t.Helper()

	return runStdin(t, env, "", args...)
}

func runStdin(t *testing.T, env []string, stdin string, args ...string) (string, string, int) {
	t.Helper()

	c := exec.Command(binPath, args...)
	c.Env = append(os.Environ(), env...)

	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}

	var stdout, stderr strings.Builder
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	code := 0

	var exitErr *exec.ExitError

	switch {
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("run %v: %v", args, err)
	}

	return stdout.String(), stderr.String(), code
}

// Happy path: load dotenv → child sees the variable.
func TestE2ELoadDotenvChildSeesVar(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("E2E_HAPPY=loaded-ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, nil, "l", "d", "-f", envFile, "--", "sh", "-c", "echo VALUE=$E2E_HAPPY")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "VALUE=loaded-ok") {
		t.Fatalf("child did not see loaded var; stdout=%q stderr=%q", stdout, stderr)
	}
}

// Happy path: load text (json) → child sees the variable.
func TestE2ELoadTextJSON(t *testing.T) {
	stdout, stderr, code := runStdin(t, nil, `{"E2E_JSON_VAR":"from-json"}`, "l", "text", "-f", "json", "--", "sh", "-c", "echo GOT=$E2E_JSON_VAR")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "GOT=from-json") {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

// Bug fix regression: child's non-zero exit code must propagate (was always 1).
func TestE2EExitCodePropagation(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, code := run(t, nil, "l", "d", "-f", envFile, "--", "sh", "-c", "exit 7")
	if code != 7 {
		t.Fatalf("want exit 7 (real child code), got %d", code)
	}
}

// Bad path: missing dotenv file → non-zero exit.
func TestE2EMissingFileFails(t *testing.T) {
	_, _, code := run(t, nil, "l", "d", "-f", "/nonexistent/nope.env", "--", "true")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing .env file")
	}
}

// Edge: existing env var wins without --override.
func TestE2ENoOverrideExistingWins(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("E2E_PRESET=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := run(t, []string{"E2E_PRESET=from-env"}, "l", "d", "-f", envFile, "--", "sh", "-c", "echo P=$E2E_PRESET")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "P=from-env") {
		t.Fatalf("existing env var should win without --override; stdout=%q", stdout)
	}
}

// Edge: --override makes the file win.
func TestE2EOverrideFileWins(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("E2E_PRESET2=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := run(t, []string{"E2E_PRESET2=from-env"}, "l", "d", "--override", "-f", envFile, "--", "sh", "-c", "echo P=$E2E_PRESET2")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "P=from-file") {
		t.Fatalf("--override should make file win; stdout=%q", stdout)
	}
}

// Bug fix regression: write dotenv --target writes the target file.
func TestE2EWriteDotenvTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.env")
	target := filepath.Join(dir, "out.env")

	if err := os.WriteFile(source, []byte("WRITTEN_KEY=written-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := run(t, nil, "w", "-s", source, "d", "-t", target)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("target file not written: %v (stderr=%s)", err, stderr)
	}
	if !strings.Contains(string(content), "WRITTEN_KEY") {
		t.Fatalf("target content=%q", content)
	}
}

func TestE2EUnknownSubcommandFails(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "load",
			args: []string{"load", "definitely-not-a-provider"},
		},
		{
			name: "write",
			args: []string{"write", "definitely-not-a-provider"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := run(t, nil, tt.args...)
			if code == 0 {
				t.Fatalf("expected non-zero exit; stdout=%q stderr=%q", stdout, stderr)
			}

			output := stdout + stderr
			if !strings.Contains(output, "unknown command") ||
				!strings.Contains(output, "definitely-not-a-provider") {
				t.Fatalf("expected unknown command error; stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

// Version prints something.
func TestE2EVersion(t *testing.T) {
	stdout, stderr, code := run(t, nil, "version")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if strings.TrimSpace(stdout+stderr) == "" {
		t.Fatal("version printed nothing")
	}
}
