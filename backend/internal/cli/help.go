package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"ircord/backend/internal/ui"
)

type helpEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group,omitempty"`
}

func renderHelp(cmd *cobra.Command, p ui.Printer) {
	commands, options := []helpEntry{}, []helpEntry{}
	for _, c := range cmd.Commands() {
		if !c.Hidden {
			commands = append(commands, helpEntry{c.Name(), c.Short, c.GroupID})
		}
	}
	cmd.InheritedFlags() // Merge inherited options for child help.
	flags := pflag.NewFlagSet("help", pflag.ContinueOnError)
	flags.AddFlagSet(cmd.LocalFlags())
	flags.AddFlagSet(cmd.InheritedFlags())
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		label := "--" + f.Name
		if f.Value.Type() != "bool" {
			label += " <" + f.Value.Type() + ">"
		}
		options = append(options, helpEntry{label, f.Usage, ""})
	})
	usage := cmd.UseLine()
	if cmd == cmd.Root() {
		usage = "ircord <command> [options]"
	}
	description := cmd.Long
	if description == "" {
		description = cmd.Short
	}
	if p.JSON {
		_ = p.Encode(struct {
			Name        string      `json:"name"`
			Usage       string      `json:"usage"`
			Description string      `json:"description"`
			Commands    []helpEntry `json:"commands"`
			Options     []helpEntry `json:"options"`
			Examples    string      `json:"examples,omitempty"`
		}{cmd.CommandPath(), usage, description, commands, options, cmd.Example})
		return
	}
	p.Heading(cmd.CommandPath() + " — " + cmd.Short)
	fmt.Fprintln(p.Out, "\n"+description)
	p.Heading("Usage")
	fmt.Fprintln(p.Out, "  "+usage)
	for _, group := range cmd.Groups() {
		p.Heading(group.Title)
		for _, entry := range commands {
			if entry.Group == group.ID {
				p.Row(entry.Name, entry.Description)
			}
		}
	}
	ungrouped := false
	for _, entry := range commands {
		if entry.Group == "" {
			if !ungrouped {
				p.Heading("Commands")
				ungrouped = true
			}
			p.Row(entry.Name, entry.Description)
		}
	}
	if len(options) > 0 {
		p.Heading("Options")
		for _, entry := range options {
			fmt.Fprintf(p.Out, "  %-29s %s\n", entry.Name, entry.Description)
		}
	}
	if cmd.Example != "" {
		p.Heading("Examples")
		fmt.Fprintln(p.Out, "  "+strings.ReplaceAll(cmd.Example, "\n", "\n  "))
	}
	fmt.Fprintln(p.Out, "\nRun 'ircord <command> --help' for command details.")
}
