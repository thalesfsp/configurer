package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kvz/logstreamer"
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

// Regex pattern for .env extensions (.env, .env.local, .env.prod, etc.)
var envRegex = regexp.MustCompile(`^\.env(\..+)?$`)

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
	// Addresses is the list of addresses to connect to. Defaults to
	// "http://localhost:9200".
	Addresses []string `json:"addresses"`

	// APIKey is one way of authenticating to the ElasticSearch cluster using
	// ElasticSearch API Key.
	APIKey string `json:"apiKey,omitempty"`

	// CloudID is one way of authenticating to the ElasticSearch cluster using
	// Elastic Cloud.
	CloudID string `json:"cloudId,omitempty"`

	// FlushInterval is the interval at which the buffer is flushed. Defaults to
	// 1 second.
	FlushInterval time.Duration `json:"flushInterval,omitempty"`

	// Index to write events to. Defaults to "configurer".
	Index string `json:"index"`

	// Password and Username are one way of authenticating to the ElasticSearch
	// cluster.
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"`

	// ServiceToken is one way of authenticating to the ElasticSearch cluster.
	ServiceToken string `json:"serviceToken,omitempty"`
}

// Run the command and properly handle signals.
//
//nolint:funlen,nestif,gocognit
func runCommand(
	p provider.IProvider,
	command string,
	arguments []string,
	combinedOutput bool,
) int {
	// The structured command to run.
	c := exec.Command(command, arguments...)

	// Builds the command and arguments string - for logging purposes only.
	cmdAndArgs := command + " " + strings.Join(arguments, " ")

	// Default log outputs.
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	// Default combined settings.
	if combinedOutput {
		finalCmdAndArgs := cmdAndArgs

		if len(arguments) > 0 {
			finalCmdAndArgs += " "
		}

		logger := log.New(os.Stdout, finalCmdAndArgs, log.LstdFlags)

		// Setup a streamer that will pipe `stderr`.
		logStreamerErr := logstreamer.NewLogstreamer(logger, "stderr", true)
		defer logStreamerErr.Close()

		// Setup a streamer that will pipe to `stdout`.
		logStreamerOut := logstreamer.NewLogstreamer(logger, "stdout", false)
		defer logStreamerOut.Close()

		c.Stderr = logStreamerErr
		c.Stdout = logStreamerOut
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

	//////
	// Redirect output.
	//////

	logOutputsStr := strings.Join(logOutputs, ",")

	if logOutputsStr != "" {
		cliLogger.Debugln("directing output to", logOutputsStr)
	}

	if strings.ContainsAny(logOutputsStr, "elasticsearch") {
		bufStdOut := new(bytes.Buffer)
		bufStdErr := new(bytes.Buffer)

		cliLogger.Debugln("setting up elasticsearch output")

		var esConfig ElasticSearchConfig

		if logSettings == "" {
			// `Fatal` instead of `1` because it's a configuration error, no the
			// command's error.
			cliLogger.Fatalln("missing log settings")
		}

		if err := json.Unmarshal([]byte(logSettings), &esConfig); err != nil {
			// `Fatal` instead of `1` because it's a configuration error, no the
			// command's error.
			cliLogger.Fatalln("failed to parse log settings", err)
		}

		// Validation.
		if esConfig.Index == "" {
			cliLogger.Fatalln("elasticsearch output is specified but index is missing")
		}

		if len(esConfig.Addresses) == 0 {
			cliLogger.Warnln("elasticsearch output is specified but addresses are missing. Using default address: http://localhost:9200")

			esConfig.Addresses = []string{"http://localhost:9200"}
		}

		// Set default flush interval.
		if esConfig.FlushInterval == 0 {
			esConfig.FlushInterval = 1 * time.Second
		}

		esOutput := output.ElasticSearchWithDynamicIndex(
			func() string {
				return fmt.Sprintf("%s-%s", esConfig.Index, time.Now().Format("2006-01"))
			},
			output.ElasticSearchConfig{
				Addresses:    esConfig.Addresses,
				APIKey:       esConfig.APIKey,
				CloudID:      esConfig.CloudID,
				Password:     esConfig.Password,
				ServiceToken: esConfig.ServiceToken,
				Username:     esConfig.Username,
			},
			level.Info,
			// Force the output to be printed.
			processor.Flagger(flag.Force),
		)

		l := sypl.New(cmdAndArgs, esOutput).SetFields(fields.Fields{
			"command": command,
			"args":    arguments,
		})

		cliLogger.Debugln("elasticsearch output set up")

		// Builds the prefix.
		if combinedOutput {
			for _, o := range l.GetOutputs() {
				o.AddProcessors(processor.Tagger("combined"))
			}
		}

		l.SetDefaultIoWriterLevel(level.Info)

		// Create a multi-writer for Stdout
		stdoutMultiWriter := io.MultiWriter(os.Stdout, bufStdOut)

		// Create a multi-writer for Stderr
		stderrMultiWriter := io.MultiWriter(os.Stderr, bufStdErr)

		c.Stdin = os.Stdin
		c.Stdout = stdoutMultiWriter
		c.Stderr = stderrMultiWriter

		// Start a goroutine for periodic flushing.
		go func() {
			// Flush every 1 second.
			for {
				select {
				case <-stop:
					return
				default:
					// Only if there is something to flush.
					if bufStdOut.Len() > 0 {
						// Read forever til end of line or error.
						for {
							line, err := bufStdOut.ReadString('\n')
							if err != nil {
								if err != io.EOF {
									l.PrintWithOptions(
										level.Error,
										"failed to read stdout buffer",
										sypl.WithField("error", err),
										sypl.WithField("line", line),
									)
								}

								// Break only in case of EOF, continue loop otherwise
								break
							}

							l.Info(line)

							cliLogger.Debugln("flushed stdout", len(line), "buffer")
						}
					}

					// Only if there is something to flush.
					if bufStdErr.Len() > 0 {
						// Read forever til end of line or error.
						for {
							line, err := bufStdErr.ReadString('\n')
							if err != nil {
								if err != io.EOF {
									l.PrintWithOptions(
										level.Error,
										"failed to read stderr buffer",
										sypl.WithField("error", err),
										sypl.WithField("line", line),
									)
								}

								// Break only in case of EOF, continue loop otherwise
								break
							}

							l.Error(line)

							cliLogger.Debugln("flushed stderr", len(line), "buffer")
						}
					}

					time.Sleep(esConfig.FlushInterval)
				}
			}
		}()
	}

	// Start command and wait it to finish.
	if err := c.Run(); err != nil {
		// If an error occurs, handle it appropriately.
		// For example, log the error and return a non-zero status.
		cliLogger.Errorlnf("error running command: %s", err)

		return 1
	}

	// Handle non-zero exit codes
	handleNonZeroExit(p, c, cmdAndArgs)

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
		cliLogger.Errorlnf(
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
			cliLogger.Errorlnf(
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

	// Run sequentially.
	if execMode == "sequential" {
		for _, c := range ca {
			if exitCode := runCommand(p, c.Command, c.Args, true); exitCode != 0 {
				os.Exit(exitCode)
			}

			cliLogger.Debuglnf("Command run successfully, waiting %s for the next command to run", sequentialDelay)

			time.Sleep(sequentialDelay)

			cliLogger.Debuglnf("Waited successfully, running the next command")
		}

		os.Exit(0)
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
		if p != nil {
			p.GetLogger().PrintlnPretty(level.Error, errs)
		} else {
			cliLogger.PrintlnPretty(level.Error, errs)
		}

		os.Exit(1)
	}

	os.Exit(0)
}

// DumpToFile dumps the final loaded values to a file. Extension is used to
// determine the format.
func DumpToFile(file *os.File, finalValues map[string]string, rawValue bool) error {
	extension := filepath.Ext(file.Name())

	switch {
	case envRegex.MatchString(extension):
		if err := util.DumpToEnv(file, finalValues, rawValue); err != nil {
			return err
		}
	case extension == ".json":
		if err := util.DumpToJSON(file, finalValues); err != nil {
			return err
		}
	case extension == ".yaml" || extension == ".yml":
		if err := util.DumpToYAML(file, finalValues); err != nil {
			return err
		}
	default:
		log.Fatalln("invalid file extension, allowed: .env.*, .json, .yaml | .yml")
	}

	return nil
}

// ParseFile parse file. Extension is used to determine the format.
func ParseFile(ctx context.Context, file *os.File) (map[string]any, error) {
	extension := filepath.Ext(file.Name())

	switch {
	case envRegex.MatchString(extension):
		p, err := env.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, file)
		if err != nil {
			return nil, err
		}

		return r, nil
	case extension == ".json":
		p, err := jsonp.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, file)
		if err != nil {
			return nil, err
		}

		return r, nil
	case extension == ".yaml" || extension == ".yml":
		p, err := env.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, file)
		if err != nil {
			return nil, err
		}

		return r, nil
	case extension == ".toml":
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
			NewInvalidError("file extension, allowed: .env.*, .json, .yaml | .yml, .toml")
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

	os.Exit(runCommand(nil, command, arguments, false))
}
