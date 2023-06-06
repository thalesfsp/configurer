package cmd

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kvz/logstreamer"
	"github.com/thalesfsp/configurer/parsers/env"
	"github.com/thalesfsp/configurer/parsers/jsonp"
	"github.com/thalesfsp/configurer/parsers/toml"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/configurer/util"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
)

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

// Run the command and properly handle signals.
func runCommand(
	p provider.IProvider,
	command string,
	arguments []string,
	combinedOutput bool,
) int {
	c := exec.Command(command, arguments...)

	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout

	// Builds the prefix.
	cmdArgs := strings.TrimSuffix(command+" "+strings.Join(arguments, " "), " ") + " -> "

	if combinedOutput {
		logger := log.New(os.Stdout, cmdArgs, log.LstdFlags)

		// Setup a streamer that will pipe `stderr`.
		logStreamerErr := logstreamer.NewLogstreamer(logger, "stderr", true)
		defer logStreamerErr.Close()

		// Setup a streamer that will pipe to `stdout`.
		logStreamerOut := logstreamer.NewLogstreamer(logger, "stdout", false)
		defer logStreamerOut.Close()

		c.Stderr = logStreamerErr
		c.Stdout = logStreamerOut
	}

	// Should kill the command after the specified timeout, and if received
	// a SIGINT.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop

		time.Sleep(shutdownTimeout)

		c.Process.Kill()

		p.GetLogger().Infolnf(
			"command killed after exceeding timeout of %s",
			shutdownTimeout,
		)

		os.Exit(1)
	}()

	// Start command and wait it to finish.
	if err := c.Run(); err != nil {
		return 1
	}

	c.Wait()

	if c.ProcessState.ExitCode() != 0 {
		p.GetLogger().PrintWithOptions(
			level.Error,
			"command exited with non-zero exit code",
			sypl.WithFields(map[string]interface{}{
				"command":  cmdArgs,
				"exitCode": c.ProcessState.ExitCode(),
			}),
		)
	}

	// Should exit with the same exit code as the command.
	return c.ProcessState.ExitCode()
}

// ConcurrentRunner runs the commands concurrently.
func ConcurrentRunner(p provider.IProvider, cmds []string, args []string) []int {
	var wg sync.WaitGroup

	exitCodes := make([]int, 0)

	if len(cmds) == 0 {
		wg.Add(1)

		command, arguments := splitCmdFromArgs(args)

		exitCodes = append(exitCodes, runCommand(p, command, arguments, false))

		wg.Done()
	} else {
		// Iterate over the commands and run them concurrently. WAIT for all
		// of them to finish before exiting, then exit the application.
		for _, command := range cmds {
			command := command

			wg.Add(1)

			// Split command from arguments.
			cmdArgs := strings.Split(command, " ")

			c, a := splitCmdFromArgs(cmdArgs)

			go func() {
				exitCodes = append(exitCodes, runCommand(p, c, a, true))

				wg.Done()
			}()
		}
	}

	wg.Wait()

	return exitCodes
}

// DumpToFile dumps the final loaded values to a file. Extension is used to
// determine the format.
func DumpToFile(file *os.File, finalValues map[string]string) error {
	extension := filepath.Ext(file.Name())

	switch extension {
	case ".env":
		if err := util.DumpToEnv(file, finalValues); err != nil {
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
