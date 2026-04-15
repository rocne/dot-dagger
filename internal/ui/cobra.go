package ui

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// SetupCobraColors installs colored help/usage templates on cmd.
// Call on the root command before Execute(). No-op when color is disabled.
func SetupCobraColors(cmd *cobra.Command) {
	cobra.AddTemplateFunc("colorHeader", Header)
	cobra.AddTemplateFunc("colorFlagUsages", colorFlagUsages)
	cobra.AddTemplateFunc("colorCmdName", colorCmdName)

	cmd.SetHelpTemplate(colorHelpTemplate)
	cmd.SetUsageTemplate(colorUsageTemplate)
}

// colorFlagUsages highlights --flag and -f patterns in the FlagUsages string.
// Uses regexp on the pre-formatted string since pflag returns it as one block.
var flagRe = regexp.MustCompile(`(--[\w-]+|-\w\b)`)

func colorFlagUsages(s string) string {
	return flagRe.ReplaceAllStringFunc(s, func(m string) string {
		return cyan.Sprint(m)
	})
}

// colorCmdName colors the name and pads to width based on the original (non-ANSI)
// length, so alignment isn't broken by escape codes.
func colorCmdName(name string, padding int) string {
	colored := cyan.Sprint(name)
	pad := padding - len(name)
	if pad < 0 {
		pad = 0
	}
	return colored + strings.Repeat(" ", pad)
}

const colorHelpTemplate = `{{with .Long}}{{. | trimRightSpace}}

{{end}}{{colorHeader "Usage:"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{colorHeader "Aliases:"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{colorHeader "Examples:"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

{{colorHeader "Available Commands:"}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{colorCmdName .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{colorHeader "Flags:"}}
{{.LocalFlags.FlagUsages | trimRightSpace | colorFlagUsages}}{{end}}{{if .HasAvailableInheritedFlags}}

{{colorHeader "Global Flags:"}}
{{.InheritedFlags.FlagUsages | trimRightSpace | colorFlagUsages}}{{end}}{{if .HasHelpSubCommands}}

{{colorHeader "Additional help topics:"}}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{colorCmdName .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

const colorUsageTemplate = `{{colorHeader "Usage:"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}

{{if gt (len .Aliases) 0}}{{colorHeader "Aliases:"}}
  {{.NameAndAliases}}

{{end}}{{if .HasExample}}{{colorHeader "Examples:"}}
{{.Example}}

{{end}}{{if .HasAvailableSubCommands}}{{colorHeader "Available Commands:"}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{colorCmdName .Name .NamePadding}} {{.Short}}{{end}}{{end}}

{{end}}{{if .HasAvailableLocalFlags}}{{colorHeader "Flags:"}}
{{.LocalFlags.FlagUsages | trimRightSpace | colorFlagUsages}}{{end}}

{{if .HasAvailableInheritedFlags}}{{colorHeader "Global Flags:"}}
{{.InheritedFlags.FlagUsages | trimRightSpace | colorFlagUsages}}{{end}}{{if .HasHelpSubCommands}}

{{colorHeader "Additional help topics:"}}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{colorCmdName .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
