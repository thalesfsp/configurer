package cmd

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kvz/logstreamer"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/configurer/util"
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
) int {
	c := exec.Command(command, arguments...)

	// Builds the prefix.
	cmdArgs := strings.TrimSuffix(command+" "+strings.Join(arguments, " "), " ") + " -> "

	//////
	// Create a logger streamer for both stdout and stderr.
	//////

	logger := log.New(os.Stdout, cmdArgs, log.LstdFlags)

	// Setup a streamer that will pipe to `stdout`.
	logStreamerOut := logstreamer.NewLogstreamer(logger, "stdout", false)
	defer logStreamerOut.Close()

	// Setup a streamer that will pipe `stderr`.
	logStreamerErr := logstreamer.NewLogstreamer(logger, "stderr", true)
	defer logStreamerErr.Close()

	c.Stdout = logStreamerOut
	c.Stderr = logStreamerErr
	c.Stdin = os.Stdin

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
		log.Fatalln(err)
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

		exitCodes = append(exitCodes, runCommand(p, command, arguments))

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
				exitCodes = append(exitCodes, runCommand(p, c, a))

				wg.Done()
			}()
		}
	}

	wg.Wait()

	return exitCodes
}

// DumpToFile dumps the final loaded values to a file. Extension is used to
// determine the format.
func DumpToFile(filename string, finalValues map[string]string) error {
	// Should be able to dump the loaded values to a file.
	if filename == "" {
		return nil
	}

	extension := filepath.Ext(filename)

	switch extension {
	case ".env":
		if err := util.DumpToEnv(filename, finalValues); err != nil {
			return err
		}
	case ".json":
		if err := util.DumpToJSON(filename, finalValues); err != nil {
			return err
		}
	case ".yaml", ".yml":
		if err := util.DumpToYAML(filename, finalValues); err != nil {
			return err
		}
	default:
		log.Fatalln("invalid file extension, allowed: .env, .json, .yaml | .yml")
	}

	return nil
}
