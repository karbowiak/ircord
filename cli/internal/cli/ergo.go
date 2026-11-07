package cli

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ircord/cli/internal/adminclient"
	"ircord/cli/internal/contract"
	"ircord/cli/internal/ui"
)

// Probe only when resolving help/completion/integration commands. Missing
// credentials, old servers, and offline operation retain the base command tree.
// No backend-only environment variables or vendor credentials are read here.
func discoverErgo(ctx context.Context, args []string, getenv func(string) string) bool {
	probe := &cobra.Command{}
	var flags remoteFlags
	f := probe.Flags()
	f.SetOutput(io.Discard)
	f.ParseErrorsWhitelist.UnknownFlags = true
	f.StringVar(&flags.server, "server", "", "")
	f.StringVar(&flags.config, "config", "", "")
	f.StringVar(&flags.tokenFile, "admin-token-file", "", "")
	f.DurationVar(&flags.timeout, "timeout", 10*time.Second, "")
	f.Bool("json", false, "")
	f.Bool("no-color", false, "")
	f.Bool("version", false, "")
	f.BoolP("help", "h", false, "")
	if f.Parse(args) != nil {
		return false
	}
	version, _ := f.GetBool("version")
	if version {
		return false
	}
	positionals := f.Args()
	wanted := len(positionals) == 0
	if len(positionals) > 0 {
		switch positionals[0] {
		case "help", "completion", "ergo", "__complete", "__completeNoDesc":
			wanted = true
		}
	}
	if !wanted {
		return false
	}
	client, err := flags.client(probe, getenv)
	if err != nil {
		return false
	}
	defer client.Close()
	deadline, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	capabilities, err := client.Capabilities(deadline)
	if err != nil {
		return false
	}
	for _, feature := range capabilities.Features {
		if feature == "ergo" {
			return true
		}
	}
	return false
}

func remoteCommand[T any](use, description string, args cobra.PositionalArgs, flags *remoteFlags, getenv func(string) string, printer func() ui.Printer,
	fetch func(context.Context, *adminclient.Client, []string) (T, error), render func(ui.Printer, T), healthy func(T) bool) *cobra.Command {
	return &cobra.Command{Use: use, Short: description, GroupID: "admin", Args: args, RunE: func(cmd *cobra.Command, args []string) error {
		client, err := flags.client(cmd, getenv)
		if err != nil {
			return err
		}
		defer client.Close()
		value, err := fetch(cmd.Context(), client, args)
		if err != nil {
			code := 4
			var apiErr *adminclient.APIError
			if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403) {
				code = 3
			}
			return &exitError{code, err}
		}
		p := printer()
		if p.JSON {
			if err := p.Encode(value); err != nil {
				return &exitError{1, errors.New("cannot write administration output")}
			}
		} else {
			render(p, value)
		}
		if healthy != nil && !healthy(value) {
			return &exitError{1, errors.New("one or more administration checks failed")}
		}
		return nil
	}}
}

func renderErgoStatus(p ui.Printer, v contract.ErgoStatus) {
	p.Heading("Ergo · " + v.Server)
	p.Row("State", string(v.State))
	p.Row("Checked", v.CheckedAt.Format(time.RFC3339))
	if v.Error != nil {
		p.Row("Reason", *v.Error)
	}
	if v.Statistics == nil {
		return
	}
	s := v.Statistics
	p.Row("Version", s.Version)
	p.Row("Go", s.GoVersion)
	p.Row("Uptime", (time.Duration(s.UptimeSeconds) * time.Second).String())
	p.Row("IRC users", strconv.FormatInt(s.Users.Total, 10))
	p.Row("Operators", strconv.FormatInt(s.Users.Operators, 10))
	p.Row("Peak users", strconv.FormatInt(s.Users.Peak, 10))
	p.Row("Channels", strconv.FormatInt(s.Channels, 10))
}

func ergoCommands(flags *remoteFlags, getenv func(string) string, printer func() ui.Printer) *cobra.Command {
	root := &cobra.Command{Use: "ergo", Short: "Inspect the backend's associated IRC server", GroupID: "admin"}
	status := remoteCommand("status", "Inspect associated-server statistics", cobra.NoArgs, flags, getenv, printer,
		func(ctx context.Context, c *adminclient.Client, _ []string) (contract.ErgoStatus, error) {
			return c.ErgoStatus(ctx)
		}, renderErgoStatus, func(v contract.ErgoStatus) bool { return v.State == "ok" && v.Statistics != nil })
	status.GroupID = ""
	root.AddCommand(status)
	channels := &cobra.Command{Use: "channels", Short: "Inspect associated-server channels"}
	list := remoteCommand("list", "List channels and membership counts", cobra.NoArgs, flags, getenv, printer,
		func(ctx context.Context, c *adminclient.Client, _ []string) (contract.ItemsErgoChannel, error) {
			return c.ErgoChannels(ctx)
		},
		func(p ui.Printer, v contract.ItemsErgoChannel) {
			p.Heading("Channels")
			for _, ch := range v.Items {
				p.Row(ch.Name, strconv.FormatInt(ch.Users, 10)+" users · "+ch.Topic)
			}
		}, nil)
	list.GroupID = ""
	channels.AddCommand(list)
	root.AddCommand(channels)
	users := &cobra.Command{Use: "users", Short: "Inspect an IRC nickname"}
	show := remoteCommand("show <nickname>", "Inspect nickname presence and sessions", cobra.ExactArgs(1), flags, getenv, printer,
		func(ctx context.Context, c *adminclient.Client, args []string) (contract.ErgoUser, error) {
			return c.ErgoUser(ctx, args[0])
		},
		func(p ui.Printer, v contract.ErgoUser) {
			p.Heading(v.Nickname)
			if v.Account != nil {
				p.Row("IRC account", *v.Account)
			}
			if v.Realname != nil {
				p.Row("Real name", *v.Realname)
			}
			p.Row("Sessions", strconv.FormatInt(v.Sessions, 10))
			for _, ch := range v.Channels {
				p.Row("Channel", ch.Name)
			}
		}, nil)
	show.GroupID = ""
	users.AddCommand(show)
	root.AddCommand(users)
	accounts := &cobra.Command{Use: "accounts", Short: "Inspect upstream NickServ accounts"}
	accountList := remoteCommand("list", "List NickServ accounts", cobra.NoArgs, flags, getenv, printer,
		func(ctx context.Context, c *adminclient.Client, _ []string) (contract.ItemsErgoAccount, error) {
			return c.ErgoAccounts(ctx)
		},
		func(p ui.Printer, v contract.ItemsErgoAccount) {
			p.Heading("NickServ accounts")
			for _, a := range v.Items {
				p.Row(a.Name, strings.Join(a.Channels, ", "))
			}
		}, nil)
	accountList.GroupID = ""
	accountShow := remoteCommand("show <account>", "Inspect a NickServ account", cobra.ExactArgs(1), flags, getenv, printer,
		func(ctx context.Context, c *adminclient.Client, args []string) (contract.ErgoAccount, error) {
			return c.ErgoAccount(ctx, args[0])
		},
		func(p ui.Printer, v contract.ErgoAccount) {
			p.Heading(v.Name)
			if v.RegisteredAt != nil {
				p.Row("Registered", v.RegisteredAt.Format(time.RFC3339))
			}
			p.Row("Channels", strings.Join(v.Channels, ", "))
		}, nil)
	accountShow.GroupID = ""
	accounts.AddCommand(accountList, accountShow)
	root.AddCommand(accounts)
	return root
}
