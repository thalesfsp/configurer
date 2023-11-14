package cmd

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/thalesfsp/concurrentloop"
	"github.com/thalesfsp/configurer/parsers/env"
	"github.com/thalesfsp/configurer/parsers/jsonp"
	"github.com/thalesfsp/configurer/parsers/toml"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/configurer/util"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/mole/core"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/fields"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/sypl/output"
	"github.com/thalesfsp/sypl/processor"
)

// CommandArgs represents the command and its arguments.
type CommandArgs struct {
	// Arguments to pass to the command.
	Args []string `json:"args"`

	// Command to run.
	Command string `json:"command"`
}

// Splits the command from the arguments.
func splitCmdFromArgs(args []string) (string, []string) {
	var (
		command   string
		arguments []string
	)

	for i := 0; i < len(args); i++ {
		if i == 0 {
			command = args[i]
		} else {
			arguments = append(arguments, args[i])
		}
	}

	return command, arguments
}

// ElasticSearchConfig represents the ElasticSearch configuration.
type ElasticSearchConfig struct {
	Addresses []string `json:"addresses"`
	APIKey    string   `json:"apiKey,omitempty"`
	CloudID   string   `json:"cloudId,omitempty"`
	Index     string   `json:"index"`
	Password  string   `json:"password,omitempty"`
	Username  string   `json:"username,omitempty"`
}

// runCommand executes a command with given arguments and streams the output.
func runCommand(p provider.IProvider, command string, arguments []string, combinedOutput bool) int {
	c := exec.Command(command, arguments...)

	// Setting up stdout and stderr pipe
	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		log.Fatal("Failed to create stdout pipe", err)
	}
	stderrPipe, err := c.StderrPipe()
	if err != nil {
		log.Fatal("Failed to create stderr pipe", err)
	}

	// Logger setup
	l := sypl.New("configurer", []output.IOutput{
		output.New("stdout", level.Trace, os.Stdout, processor.MuteBasedOnLevel(level.Fatal, level.Error)),
		output.New("stderr", level.Error, os.Stderr),
	}...).SetFields(fields.Fields{
		"command": command,
		"args":    arguments,
	})

	// Logger processors setup
	for _, o := range l.GetOutputs() {
		o.AddProcessors(processor.ChangeFirstCharCase(processor.Lowercase))
	}

	// Start command
	if err := c.Start(); err != nil {
		log.Fatal("Failed to start command", err)
	}

	// Stream stdout
	go streamOutput(p, stdoutPipe, "stdout")
	// Stream stderr
	go streamOutput(p, stderrPipe, "stderr")

	// Signal handling setup
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop

		// Assuming shutdownTimeout is a predefined duration
		time.Sleep(shutdownTimeout)

		c.Process.Kill()

		handleCommandKill(p)
	}()

	// Wait for command to finish
	if err := c.Wait(); err != nil {
		handleNonZeroExit(p, c, command+" "+strings.Join(arguments, " "))
		return 1
	}

	// Handle non-zero exit codes
	handleNonZeroExit(p, c, command+" "+strings.Join(arguments, " "))

	return c.ProcessState.ExitCode()
}

// streamOutput reads from a pipe and logs each line.
func streamOutput(p provider.IProvider, pipe io.ReadCloser, outputType string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		// Log the output line
		if p != nil {
			p.GetLogger().PrintWithOptions(
				level.Info,
				line,
				sypl.WithFields(map[string]interface{}{
					"outputType": outputType,
				}),
			)
		} else {
			log.Printf("[%s] %s", outputType, line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading %s: %s", outputType, err)
	}
}

func handleCommandKill(p provider.IProvider) {
	if p != nil {
		p.GetLogger().Infolnf(
			"command killed after exceeding timeout of %s",
			shutdownTimeout,
		)
	} else {
		log.Printf(
			"command killed after exceeding timeout of %s",
			shutdownTimeout,
		)
	}

	os.Exit(1)
}

func handleNonZeroExit(p provider.IProvider, c *exec.Cmd, cmdArgs string) {
	if c.ProcessState.ExitCode() != 0 {
		if p != nil {
			p.GetLogger().PrintWithOptions(
				level.Error,
				"command exited with non-zero exit code",
				sypl.WithFields(map[string]interface{}{
					"command":  cmdArgs,
					"exitCode": c.ProcessState.ExitCode(),
				}),
			)
		} else {
			log.Printf(
				"command exited with non-zero exit code, command: %s, exitCode: %d",
				cmdArgs,
				c.ProcessState.ExitCode(),
			)
		}
	}
}

// ConcurrentRunner runs the commands concurrently.
func ConcurrentRunner(p provider.IProvider, cmds []string, args []string) {
	if len(cmds) == 0 {
		command, arguments := splitCmdFromArgs(args)

		os.Exit(runCommand(p, command, arguments, false))
	}

	ca := []CommandArgs{}

	for _, command := range cmds {
		// Split command from arguments.
		cmdArgs := strings.Split(command, " ")

		c, a := splitCmdFromArgs(cmdArgs)

		ca = append(ca, CommandArgs{
			Command: c,
			Args:    a,
		})
	}

	if _, errs := concurrentloop.Map(context.Background(), ca, func(ctx context.Context, ca CommandArgs) (bool, error) {
		if exitCode := runCommand(p, ca.Command, ca.Args, true); exitCode != 0 {
			return false, customerror.NewFailedToError(
				"run command",
				customerror.WithField("command", ca.Command),
				customerror.WithField("args", ca.Args),
			)
		}

		return true, nil
	}); len(errs) > 0 {
		p.GetLogger().PrintlnPretty(level.Error, errs)

		os.Exit(1)
	}

	os.Exit(0)
}

// DumpToFile dumps the final loaded values to a file. Extension is used to
// determine the format.
func DumpToFile(file *os.File, finalValues map[string]string, rawValue bool) error {
	extension := filepath.Ext(file.Name())

	switch extension {
	case ".env":
		if err := util.DumpToEnv(file, finalValues, rawValue); err != nil {
			return err
		}
	case ".json":
		if err := util.DumpToJSON(file, finalValues); err != nil {
			return err
		}
	case ".yaml", ".yml":
		if err := util.DumpToYAML(file, finalValues); err != nil {
			return err
		}
	default:
		log.Fatalln("invalid file extension, allowed: .env, .json, .yaml | .yml")
	}

	return nil
}

// ParseFile parse file. Extension is used to determine the format.
func ParseFile(ctx context.Context, file *os.File) (map[string]any, error) {
	extension := filepath.Ext(file.Name())

	switch extension {
	case ".env":
		p, err := env.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, file)
		if err != nil {
			return nil, err
		}

		return r, nil
	case ".json":
		p, err := jsonp.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, file)
		if err != nil {
			return nil, err
		}

		return r, nil
	case ".yaml", ".yml":
		p, err := env.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, file)
		if err != nil {
			return nil, err
		}

		return r, nil
	case ".toml":
		t, err := toml.New()
		if err != nil {
			return nil, err
		}

		r, err := t.Read(ctx, file)
		if err != nil {
			return nil, err
		}

		return r, nil
	default:
		return nil, customerror.
			NewInvalidError("file extension, allowed: .env, .json, .yaml | .yml, .toml")
	}
}

// CreateBridge creates a bridge.
func CreateBridge() {
	bridgeLogger.PrintlnWithOptions(
		level.Info,
		"Creating bridge",
		sypl.WithField("source", conf.Source.String()),
		sypl.WithField("destination", conf.Destination.String()),
	)

	if conf.Insecure {
		bridgeLogger.Infoln("Ignoring machine's `known_hosts` file (`insecure` is set to `true`)")
	}

	client := core.New(conf)

	if err := client.Start(); err != nil {
		bridgeLogger.Fatalln("failed to start client", err)
	}
}

// ValidateConnection validates the connection.
func ValidateConnection() {
	bridgeLogger.Infolnf("Validating connection (set `validate-connection` to `false` to disable this)")

	attempts := 0

	maxAttempts := bridgeRetryMaxAttempts

	for {
		conn, err := net.Dial("tcp", conf.Source.String())
		if err == nil {
			conn.Close()

			break
		}

		attempts++

		if attempts >= maxAttempts {
			bridgeLogger.Fatallnf(
				"Failed to connect to %s, %d attempts",
				conf.Source.String(),
				maxAttempts,
			)
		}

		time.Sleep(bridgeRetryDelay)
	}

	bridgeLogger.Infoln("Connection validated!")
}

// RunnerBridge runs the bridge and command.
func RunnerBridge(args []string) {
	// If no args are provided, just create the bridge.
	if len(args) == 0 {
		go func() {
			time.Sleep(bridgePostConnectionDelay)

			if bridgeValidateConnection {
				ValidateConnection()
			}
		}()

		CreateBridge()

		return
	}

	// Create the bridge in background.
	go func() {
		CreateBridge()
	}()

	time.Sleep(bridgePostConnectionDelay)

	if bridgeValidateConnection {
		ValidateConnection()
	}

	command, arguments := splitCmdFromArgs(args)

	bridgeLogger.PrintlnWithOptions(
		level.Info,
		"Running command",
		sypl.WithField("cmd", command),
	)

	os.Exit(runCommand(nil, command, arguments, false))
}
