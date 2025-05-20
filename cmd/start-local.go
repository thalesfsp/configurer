package cmd

import (
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
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
			conf.Destination.Set(bridgeDestination)
		}

		if bridgeServer == "" && cBSe != "" {
			bridgeServer = cBSe
		}

		if bridgeServer != "" {
			conf.Server.Set(bridgeServer)
		}

		if bridgeSource == "" && cBSo != "" {
			bridgeSource = cBSo
		}

		if bridgeSource != "" {
			conf.Source.Set(bridgeSource)
		}

		if conf.Destination.String() == "" {
			log.Fatalln("error: missing required flag --destination")
		}

		if conf.Server.String() == "" {
			log.Fatalln("error: missing required flag --server")
		}

		if conf.Source.String() == "" {
			log.Fatalln("error: missing required flag --source")
		}

		if bridgeKeyValue == "" && cBK != "" {
			conf.KeyValue = cBK
		}

		// Check if key or key-value is set, they are mutually exclusive.
		if conf.KeyValue == "" && conf.Key == "" {
			log.Fatalln("error: missing required flag --key or --key-value")
		}

		// Check if key and key-value are set, they are mutually exclusive.
		if conf.KeyValue != "" && conf.Key != "" {
			log.Fatalln("error: or --key or --key-value")
		}

		conf.TunnelType = "local"

		// Parse key-value if it contains \n.
		if conf.KeyValue != "" {
			if strings.Contains(conf.KeyValue, "\\n") {
				conf.KeyValue = strings.ReplaceAll(conf.KeyValue, "\\n", "\n")
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
