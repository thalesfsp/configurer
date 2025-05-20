package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/mole/core"
)

var (
	bridgeLogger = cliLogger.New("bridge")

	bridgePostConnectionDelay time.Duration
	bridgeRetryDelay          time.Duration
	bridgeRetryMaxAttempts    int
	bridgeValidateConnection  bool

	bridgeDestination string
	bridgeKeyValue    string
	bridgeServer      string
	bridgeSource      string

	conf = &core.Configuration{}
)

// bridgeCmd represents the run command.
var bridgeCmd = &cobra.Command{
	Aliases: []string{"b"},
	Use:     "bridge",
	Short:   "Manage SSH-based bridge(s) to target(s)",
}

func init() {
	rootCmd.AddCommand(bridgeCmd)

	// Required.
	bridgeCmd.PersistentFlags().StringVar(&conf.Key, "key", "", "set server authentication key file path. Required if --key-value is not set")
	bridgeCmd.PersistentFlags().StringVar(&bridgeKeyValue, "key-value", "", "set server authentication key. Required if --key is not set.")
	bridgeCmd.PersistentFlags().StringVarP(&bridgeDestination, "destination", "d", "", "set destination endpoint address.")
	bridgeCmd.PersistentFlags().StringVarP(&bridgeServer, "server", "s", "", "set server address: [<user>@]<host>[:<port>]")
	bridgeCmd.PersistentFlags().StringVarP(&bridgeSource, "source", "u", "", "set source endpoint address. Multiple -source conf can be provided")

	// Operational.
	bridgeCmd.PersistentFlags().BoolVar(&bridgeValidateConnection, "validate-connection", true, "validate connection to the server")
	bridgeCmd.PersistentFlags().DurationVar(&bridgePostConnectionDelay, "post-connection-delay", 3*time.Second, "delay after connection is established")
	bridgeCmd.PersistentFlags().DurationVar(&bridgeRetryDelay, "retry-delay", 1*time.Second, "delay between connection retries")
	bridgeCmd.PersistentFlags().IntVar(&bridgeRetryMaxAttempts, "retry-max-attempts", 3, "maximum number of connection retries")

	// Fine-tuning.
	bridgeCmd.PersistentFlags().BoolVarP(&conf.Detach, "detach", "x", false, "run process in background")
	bridgeCmd.PersistentFlags().BoolVarP(&conf.Insecure, "insecure", "i", false, "skip host key validation when connecting to ssh server")
	bridgeCmd.PersistentFlags().BoolVarP(&conf.Rpc, "rpc", "", false, "enable the rpc server")
	bridgeCmd.PersistentFlags().BoolVarP(&conf.Verbose, "verbose", "v", false, "increase log verbosity")
	bridgeCmd.PersistentFlags().DurationVarP(&conf.KeepAliveInterval, "keep-alive-interval", "K", 10*time.Second, "time interval for keep alive packets to be sent")
	bridgeCmd.PersistentFlags().DurationVarP(&conf.Timeout, "timeout", "t", 3*time.Second, "ssh server connection timeout")
	bridgeCmd.PersistentFlags().DurationVarP(&conf.WaitAndRetry, "retry-wait", "w", 3*time.Second, "time to wait before trying to reconnect to ssh server")
	bridgeCmd.PersistentFlags().IntVarP(&conf.ConnectionRetries, "connection-retries", "R", 3, `maximum number of connection retries to the ssh server provide 0 to never give up or a negative number to disable`)
	bridgeCmd.PersistentFlags().StringVarP(&conf.RpcAddress, "rpc-address", "", "127.0.0.1:0", `set the network address of the rpc server. The default value uses a random free port to listen for requests. The full address is kept on $HOME/.mole/<id>.`)
	bridgeCmd.PersistentFlags().StringVarP(&conf.SshAgent, "ssh-agent", "A", "", "unix socket to communicate with a ssh agent")
	bridgeCmd.PersistentFlags().StringVarP(&conf.SshConfig, "config", "c", "$HOME/.ssh/config", "set config file path")

	bridgeCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
	{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
	{{.CommandPath}} [provider]{{end}}{{if gt (len .Aliases) 0}}
  
  Aliases:
	{{.NameAndAliases}}{{end}}{{if .HasExample}}
  
  Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}
  
  Available Providers:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}
  
  {{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}
  
  Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
  
  Flags:
  {{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
  
  Global Flags:
  {{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}
  
  Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
	{{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}
  
  Use "{{.CommandPath}} [provider] --help" for more information about a provider.{{end}}`)
}
