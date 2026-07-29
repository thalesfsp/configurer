package cmd

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thalesfsp/mole/core"
)

var (
	bridgePostConnectionDelay time.Duration
	bridgeRetryDelay          time.Duration
	bridgeRetryMaxAttempts    int
	bridgeValidateConnection  bool

	bridgeDestination string
	bridgeKeyValue    string
	bridgeServer      string
	bridgeSource      string

	bridgeStartConfig = &core.Configuration{}
)

// startCmd represents the run command.
var startCmd = &cobra.Command{
	Aliases: []string{"s"},
	Use:     "start",
	Short:   "Start a bridge",
	Run: func(cmd *cobra.Command, args []string) {
		cBK := os.Getenv("CONFIGURER_BRIDGE_KEY")
		cBD := os.Getenv("CONFIGURER_BRIDGE_DESTINATION")
		cBSe := os.Getenv("CONFIGURER_BRIDGE_SERVER")
		cBSo := os.Getenv("CONFIGURER_BRIDGE_SOURCE")

		if bridgeDestination == "" && cBD != "" {
			bridgeDestination = cBD
		}

		if bridgeDestination != "" {
			bridgeStartConfig.Destination.Set(bridgeDestination)
		}

		if bridgeServer == "" && cBSe != "" {
			bridgeServer = cBSe
		}

		if bridgeServer != "" {
			bridgeStartConfig.Server.Set(bridgeServer)
		}

		if bridgeSource == "" && cBSo != "" {
			bridgeSource = cBSo
		}

		if bridgeSource != "" {
			bridgeStartConfig.Source.Set(bridgeSource)
		}

		if bridgeStartConfig.Destination.String() == "" {
			log.Fatalln("error: missing required flag --destination")
		}

		if bridgeStartConfig.Server.String() == "" {
			log.Fatalln("error: missing required flag --server")
		}

		if bridgeStartConfig.Source.String() == "" {
			log.Fatalln("error: missing required flag --source")
		}

		if bridgeKeyValue != "" {
			bridgeStartConfig.KeyValue = bridgeKeyValue
		} else if cBK != "" {
			bridgeStartConfig.KeyValue = cBK
		}

		// Check if key or key-value is set, they are mutually exclusive.
		if bridgeStartConfig.KeyValue == "" && bridgeStartConfig.Key == "" {
			log.Fatalln("error: missing required flag --key or --key-value")
		}

		// Check if key and key-value are set, they are mutually exclusive.
		if bridgeStartConfig.KeyValue != "" && bridgeStartConfig.Key != "" {
			log.Fatalln("error: or --key or --key-value")
		}

		bridgeStartConfig.TunnelType = "local"

		// Parse key-value if it contains \n.
		if bridgeStartConfig.KeyValue != "" {
			if strings.Contains(bridgeStartConfig.KeyValue, "\\n") {
				bridgeStartConfig.KeyValue = strings.ReplaceAll(bridgeStartConfig.KeyValue, "\\n", "\n")
			}
		}

		RunnerBridge(args)
	},
}

func init() {
	bridgeCmd.AddCommand(startCmd)

	startCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
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
