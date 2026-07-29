//go:build unix

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	signalHelperEnv      = "CONFIGURER_SIGNAL_HELPER"
	signalHelperReady    = "CONFIGURER_CHILD_READY"
	signalHelperReceived = "CONFIGURER_CHILD_RECEIVED"
)

func TestRunCommandForwardsSIGTERM(t *testing.T) {
	testRunCommandSignal(t, syscall.SIGTERM)
}

func TestRunCommandForwardsSIGINT(t *testing.T) {
	testRunCommandSignal(t, os.Interrupt)
}

func TestRunCommandKillsChildAfterShutdownTimeout(t *testing.T) {
	const timeout = 200 * time.Millisecond

	started := time.Now()
	output, err := runSignalHelper(t, syscall.SIGTERM, "ignore", timeout)
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("expected parent to exit non-zero after killing unresponsive child")
	}

	if !strings.Contains(output, signalHelperReady) {
		t.Fatalf("child did not start: %s", output)
	}

	if elapsed < timeout {
		t.Fatalf("child was killed before shutdown timeout: elapsed %s, timeout %s", elapsed, timeout)
	}

	if elapsed > 5*time.Second {
		t.Fatalf("parent hung after shutdown timeout: elapsed %s", elapsed)
	}
}

func TestRunCommandSignalHelper(t *testing.T) {
	if os.Getenv(signalHelperEnv) == "" {
		return
	}

	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}

	if len(args) < 4 {
		t.Fatalf("missing helper arguments: %v", os.Args)
	}

	role, mode, timeoutString := args[1], args[2], args[3]

	switch role {
	case "parent":
		timeout, err := time.ParseDuration(timeoutString)
		if err != nil {
			t.Fatal(err)
		}

		shutdownTimeout = timeout
		exitCode := runCommand(
			nil,
			os.Args[0],
			[]string{
				"-test.run=^TestRunCommandSignalHelper$",
				"--",
				"child",
				mode,
				timeoutString,
			},
			false,
		)
		os.Exit(exitCode)
	case "child":
		if mode == "ignore" {
			signal.Ignore(os.Interrupt, syscall.SIGTERM)
			fmt.Println(signalHelperReady)
			select {}
		}

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		fmt.Println(signalHelperReady)
		fmt.Printf("%s:%s\n", signalHelperReceived, <-signals)
		os.Exit(0)
	default:
		t.Fatalf("unknown helper role %q", role)
	}
}

func testRunCommandSignal(t *testing.T, s os.Signal) {
	t.Helper()

	output, err := runSignalHelper(t, s, "handle", 3*time.Second)
	if err != nil {
		t.Fatalf("parent exited with error: %v\n%s", err, output)
	}

	want := fmt.Sprintf("%s:%s", signalHelperReceived, s)
	if !strings.Contains(output, want) {
		t.Fatalf("child did not receive forwarded signal %s\noutput:\n%s", s, output)
	}
}

func runSignalHelper(
	t *testing.T,
	s os.Signal,
	mode string,
	timeout time.Duration,
) (string, error) {
	t.Helper()

	command := exec.Command(
		os.Args[0],
		"-test.run=^TestRunCommandSignalHelper$",
		"--",
		"parent",
		mode,
		timeout.String(),
	)
	command.Env = append(os.Environ(), signalHelperEnv+"=1")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	command.Stderr = &stderr

	if err := command.Start(); err != nil {
		t.Fatal(err)
	}

	var (
		lines     []string
		linesLock sync.Mutex
		ready     = make(chan struct{})
		readyOnce sync.Once
	)

	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			linesLock.Lock()
			lines = append(lines, line)
			linesLock.Unlock()

			if line == signalHelperReady {
				readyOnce.Do(func() { close(ready) })
			}
		}
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		command.Wait()
		t.Fatal("timed out waiting for child readiness")
	}

	if err := command.Process.Signal(s); err != nil {
		syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		command.Wait()
		t.Fatal(err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- command.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitDone:
	case <-time.After(5 * time.Second):
		syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		waitErr = <-waitDone
	}

	<-scanDone
	linesLock.Lock()
	output := strings.Join(lines, "\n")
	linesLock.Unlock()

	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	return output, waitErr
}
