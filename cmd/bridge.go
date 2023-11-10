package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/mole/core"
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
)

var (
	bridgeLogger = sypl.NewDefault("bridge", level.Error)

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

	bridgeCmd.PersistentFlags().VarP(
		&conf.Source,
		"source",
		"",
		"set source endpoint address. Multiple -source conf can be provided",
	)

	bridgeCmd.PersistentFlags().VarP(
		&conf.Destination,
		"destination",
		"",
		"set destination endpoint address. Multiple -destination conf can be provided",
	)

	bridgeCmd.PersistentFlags().VarP(
		&conf.Server,
		"server",
		"",
		"set server address: [<user>@]<host>[:<port>]",
	)

	bridgeCmd.PersistentFlags().StringVar(
		&conf.Key,
		"key",
		"",
		"set server authentication key file path",
	)

	bridgeCmd.PersistentFlags().StringVar(
		&conf.KeyValue,
		"key-value",
		os.Getenv("CONFIGURER_BRIDGE_KEY"),
		"set server authentication key",
	)

	bridgeCmd.PersistentFlags().BoolVarP(&conf.Verbose, "verbose", "v", false, "increase log verbosity")
	bridgeCmd.PersistentFlags().BoolVarP(&conf.Insecure, "insecure", "i", false, "skip host key validation when connecting to ssh server")
	bridgeCmd.PersistentFlags().BoolVarP(&conf.Detach, "detach", "x", false, "run process in background")
	bridgeCmd.PersistentFlags().DurationVarP(&conf.KeepAliveInterval, "keep-alive-interval", "K", 10*time.Second, "time interval for keep alive packets to be sent")
	bridgeCmd.PersistentFlags().IntVarP(&conf.ConnectionRetries, "connection-retries", "R", 3, `maximum number of connection retries to the ssh server
	provide 0 to never give up or a negative number to disable`)
	bridgeCmd.PersistentFlags().StringVarP(&conf.SshConfig, "config", "c", "$HOME/.ssh/config", "set config file path")
	bridgeCmd.PersistentFlags().DurationVarP(&conf.WaitAndRetry, "retry-wait", "w", 3*time.Second, "time to wait before trying to reconnect to ssh server")
	bridgeCmd.PersistentFlags().StringVarP(&conf.SshAgent, "ssh-agent", "A", "", "unix socket to communicate with a ssh agent")
	bridgeCmd.PersistentFlags().DurationVarP(&conf.Timeout, "timeout", "t", 3*time.Second, "ssh server connection timeout")
	bridgeCmd.PersistentFlags().BoolVarP(&conf.Rpc, "rpc", "", false, "enable the rpc server")
	bridgeCmd.PersistentFlags().StringVarP(&conf.RpcAddress, "rpc-address", "", "127.0.0.1:0", `set the network address of the rpc server.
	The default value uses a random free port to listen for requests.
	The full address is kept on $HOME/.mole/<id>.`)

	bridgeCmd.MarkPersistentFlagRequired("source")
	bridgeCmd.MarkPersistentFlagRequired("destination")
	bridgeCmd.MarkPersistentFlagRequired("server")

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
  
  Use "{{.CommandPath}} [provider] --help" for more information about a provider.{{end}}
  `)
}
