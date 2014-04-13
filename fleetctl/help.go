package main

import (
	"flag"
	"fmt"
	"text/template"

	"github.com/coreos/fleet/version"
)

var (
	cmdHelp = &Command{
		Name:        "help",
		Summary:     "Show a list of commands or help for one command",
		Description: "Show a list of commands or help for one command",
		Run:         runHelp,
	}

	globalUsageTemplate  *template.Template
	commandUsageTemplate *template.Template
)

func init() {
	globalUsageTemplate = template.Must(template.New("global_usage").Parse(`
NAME:
{{printf "\t%s - %s" .Executable .Description}}

USAGE: 
{{printf "\t%s" .Executable}} [global options] <command> [command options] [arguments...]

VERSION:
{{printf "\t%s" .Version}}

COMMANDS:{{range .Commands}}
{{printf "\t%s\t%s" .Name .Summary}}{{end}}

GLOBAL OPTIONS:{{range .Flags}}
{{printf "\t--%s=%s\t%s" .Name .DefValue .Usage}}{{end}}

Global options can also be configured via upper-case environment variables prefixed with "FLEETCTL_"
For example, "some-flag" => "FLEETCTL_SOME_FLAG"

Run '{{.Executable}} help <command>' for more details on a specific command.

`[1:]))
	commandUsageTemplate = template.Must(template.New("command_usage").Parse(`
NAME:
{{printf "\t%s - %s" .Cmd.Name .Cmd.Summary}}

USAGE:
{{"\t"}}{{.Executable}} [global options] {{.Cmd.Name}} {{.Cmd.Usage}}

DESCRIPTION:
{{.Cmd.Description}}

{{if .CmdFlags}}OPTIONS:{{range .CmdFlags}}
{{printf "\t--%s=%s\t%s" .Name .DefValue .Usage}}{{end}}{{end}}

For help on global options run "{{.Executable}} help"
`[1:]))
}

func runHelp(args []string) (exit int) {
	if len(args) < 1 {
		printGlobalUsage()
		return 0
	}

	var cmd *Command

	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			break
		}
	}

	if cmd == nil {
		fmt.Println("Unrecognized command:", args[0])
		return 1
	}

	printCommandUsage(cmd)
	return 0
}

func printGlobalUsage() {
	globalUsageTemplate.Execute(out, struct {
		Executable  string
		Commands    []*Command
		Flags       []*flag.Flag
		Description string
		Version     string
	}{
		cliName,
		commands,
		getAllFlags(),
		cliDescription,
		version.Version,
	})
	out.Flush()
}

func printCommandUsage(cmd *Command) {
	commandUsageTemplate.Execute(out, struct {
		Executable string
		Cmd        *Command
		CmdFlags   []*flag.Flag
	}{
		cliName,
		cmd,
		getFlags(&cmd.Flags),
	})
	out.Flush()
}

//{{"\t"}}{{.Name | printf (print "%-" $.MaxCommandNameLength "s")}}{{"\t"}}{{.Usage}}{{end}}
//{{"\t"}}{{printf "--%-*s\t%s" $.MaxFlagNameLength (printf "%s=%s" .Name .DefValue) .Usage}}{{end}}
