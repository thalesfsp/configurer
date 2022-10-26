package cmd

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/thalesfsp/configurer/provider"
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
func runCommand(p provider.IProvider, command string, arguments []string) {
	// Should be able to call a command with the loaded secrets.
	c := exec.Command(command, arguments...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
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

	// Should exit with the same exit code as the command.
	os.Exit(c.ProcessState.ExitCode())
}
