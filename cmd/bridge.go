package cmd

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/mole/core"
)

var conf = &core.Configuration{
	ConnectionRetries: 3,
	Detach:            false,
	Insecure:          false,
	KeepAliveInterval: 10 * time.Second,
	Rpc:               false,
	RpcAddress:        "127.0.0.1:0",
	SshAgent:          "ssh-agent",
	SshConfig:         "$HOME/.ssh/config",
	Timeout:           3 * time.Second,
	Verbose:           false,
	WaitAndRetry:      3 * time.Second,
}

// bridgeCmd represents the run command.
var bridgeCmd = &cobra.Command{
	Aliases: []string{"b"},
	Use:     "bridge",
	Short:   "Creates a SSH-based bridge to the target",
	Run: func(cmd *cobra.Command, args []string) {
		// Check if key or key-value is set, they are mutually exclusive.
		if conf.KeyValue == "" && conf.Key == "" {
			log.Fatalln("error: missing required flag --key or --key-value")
		}

		// Parse key-value if it contains \n.
		if conf.KeyValue != "" {
			if strings.Contains(conf.KeyValue, "\\n") {
				conf.KeyValue = strings.ReplaceAll(conf.KeyValue, "\\n", "\n")
			}
		}

		client := core.New(conf)

		err := client.Start()
		if err != nil {
			log.Fatalln("error starting bridge:", err)
		}
	},
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

	conf.TunnelType = "local"

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
