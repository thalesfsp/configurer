package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/thalesfsp/sypl/flag"
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

// Run the command and properly handle signals.
func runCommand(
	p provider.IProvider,
	command string,
	arguments []string,
	combinedOutput bool,
	disableStdIn bool,
) int {
	c := exec.Command(command, arguments...)

	lStderr := sypl.New("configurer", []output.IOutput{
		output.New("stderr", level.Error, os.Stderr),
	}...).SetFields(fields.Fields{
		"command": command,
		"args":    arguments,
	})

	lStdout := sypl.New("configurer", []output.IOutput{
		output.New("stdout", level.Trace, os.Stdout, processor.MuteBasedOnLevel(level.Fatal, level.Error)),
	}...).SetFields(fields.Fields{
		"command": command,
		"args":    arguments,
	})

	//////
	// # ElasticSearch
	//////

	// if LogOutput contains "default"
	for _, logOutput := range logOutputs {
		if logOutput == "elasticsearch" {
			var esConfig ElasticSearchConfig

			if logSettings == "" {
				// `Fatal` instead of `1` because it's a configuration error, no the
				// command's error.
				log.Fatalln("Missing log settings")
			}

			if err := json.Unmarshal([]byte(logSettings), &esConfig); err != nil {
				// `Fatal` instead of `1` because it's a configuration error, no the
				// command's error.
				log.Fatalln("Failed to parse log settings", err)
			}

			esOutput := output.ElasticSearchWithDynamicIndex(
				func() string {
					return fmt.Sprintf("%s-%s", esConfig.Index, time.Now().Format("2006-01"))
				},
				output.ElasticSearchConfig{
					Addresses: esConfig.Addresses,
					APIKey:    esConfig.APIKey,
					CloudID:   esConfig.CloudID,
					Password:  esConfig.Password,
					Username:  esConfig.Username,
				},
				level.Trace,
				// Force the output to be printed.
				processor.Flagger(flag.Force),
			)

			lStderr.AddOutputs(esOutput)
			lStdout.AddOutputs(esOutput)
		}
	}

	lStderr.SetDefaultIoWriterLevel(level.Error)
	lStdout.SetDefaultIoWriterLevel(level.Trace)

	// Use the buffer for Stderr, Stdin, and Stdout
	if !disableStdIn {
		c.Stdin = os.Stdin
	}

	c.Stderr = lStderr
	c.Stdout = lStdout

	cmdArgs := command + " " + strings.Join(arguments, " ")

	// Builds the prefix.
	if combinedOutput {
		prefix := "Command: " + cmdArgs + "\nOutput: "

		lStderr.GetOutput("stderr").AddProcessors(processor.Prefixer(prefix), processor.Suffixer("\n"))
		lStdout.GetOutput("stdout").AddProcessors(processor.Prefixer(prefix), processor.Suffixer("\n"))
	}

	// Signal handling setup
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop

		time.Sleep(shutdownTimeout)

		c.Process.Kill()

		handleCommandKill(p)
	}()

	// Start command and wait it to finish.
	if err := c.Run(); err != nil {
		return 1
	}

	c.Wait()

	// Handle non-zero exit codes
	handleNonZeroExit(p, c, cmdArgs)

	// Exit with the same exit code as the command.
	return c.ProcessState.ExitCode()
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

		os.Exit(runCommand(p, command, arguments, false, bridgeDisableStdIn))
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
		if exitCode := runCommand(p, ca.Command, ca.Args, true, bridgeDisableStdIn); exitCode != 0 {
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
		sypl.WithField("destination", conf.Destination.String()),
		sypl.WithField("server", conf.Server.String()),
		sypl.WithField("source", conf.Source.String()),
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

	bridgeLogger.Infoln("Connection validated, you're good to go!")
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

	os.Exit(runCommand(nil, command, arguments, false, bridgeDisableStdIn))
}
