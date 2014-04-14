package main

import (
	"flag"
	"fmt"
	"strings"
	"text/template"

	"github.com/coreos/fleet/version"
)

var (
	cmdHelp = &Command{
		Name:        "help",
		Summary:     "Show a list of commands or help for one command",
		Usage:       "[COMMAND]",
		Description: "Show a list of commands or detailed help for one command",
		Run:         runHelp,
	}

	globalUsageTemplate  *template.Template
	commandUsageTemplate *template.Template
	templFuncs           = template.FuncMap{
		"descToLines": func(s string) []string {
			// trim leading/trailing whitespace and split into slice of lines
			return strings.Split(strings.Trim(s, "\n\t "), "\n")
		},
	}
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

Run "{{.Executable}} help <command>" for more details on a specific command.
`[1:]))
	commandUsageTemplate = template.Must(template.New("command_usage").Funcs(templFuncs).Parse(`
NAME:
{{printf "\t%s - %s" .Cmd.Name .Cmd.Summary}}

USAGE:
{{printf "\t%s %s %s" .Executable .Cmd.Name .Cmd.Usage}}

DESCRIPTION:
{{range $line := descToLines .Cmd.Description}}{{printf "\t%s" $line}}
{{end}}
{{if .CmdFlags}}OPTIONS:
{{range .CmdFlags}}{{printf "\t--%s=%s\t%s" .Name .DefValue .Usage}}
{{end}}
{{end}}For help on global options run "{{.Executable}} help"
`[1:]))
}

func runHelp(args []string) (exit int) {
	if len(args) < 1 {
		printGlobalUsage()
		return
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
	return
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
