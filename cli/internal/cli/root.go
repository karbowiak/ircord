// Package cli builds the standalone remote administration command tree.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ircord/cli/internal/adminclient"
	"ircord/cli/internal/buildinfo"
	"ircord/cli/internal/contract"
	"ircord/cli/internal/ui"
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func Execute(ctx context.Context, args []string, out, errOut io.Writer) int {
	if args == nil {
		args = []string{}
	}
	root, printer := newRoot(ctx, args, out, errOut, os.Getenv)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		code := 2 // Cobra parsing and configuration errors.
		var commandErr *exitError
		if errors.As(err, &commandErr) {
			code = commandErr.code
		}
		if errors.Is(err, context.Canceled) {
			code = 130
		}
		message := err.Error()
		if secret := os.Getenv("IRCORD_ADMIN_TOKEN"); len(secret) >= 16 {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
		printer().Error(message, code)
		return code
	}
	return 0
}

func newRoot(ctx context.Context, args []string, out, errOut io.Writer, getenv func(string) string) (*cobra.Command, func() ui.Printer) {
	var jsonOutput, noColor, showVersion bool
	var flags remoteFlags
	printer := func() ui.Printer {
		return ui.Printer{Out: out, Err: errOut, JSON: jsonOutput || enabledFlag(args, "json"),
			Color: !jsonOutput && !enabledFlag(args, "json") && !noColor && !enabledFlag(args, "no-color") && getenv("NO_COLOR") == "" && ui.IsTerminal(out)}
	}
	info := buildinfo.Current()
	version := func() error {
		p := printer()
		if p.JSON {
			return p.Encode(info)
		}
		p.Heading("IRCord " + info.Version)
		p.Row("Commit", info.Commit)
		p.Row("Go", info.GoVersion)
		fmt.Fprintln(out)
		return nil
	}
	root := &cobra.Command{
		Use: "ircordctl", Short: "IRCord remote administration",
		Long:         "Administer a running IRCord backend over its administration API.\nThis client runs independently and never opens the backend database or starts IRC.",
		SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if showVersion {
				return version()
			}
			return cmd.Help()
		},
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.AddGroup(&cobra.Group{ID: "admin", Title: "Administration"}, &cobra.Group{ID: "local", Title: "Local tools"})
	root.SetHelpCommandGroupID("local")
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) { renderHelp(cmd, printer()) })
	pf := root.PersistentFlags()
	pf.BoolVar(&jsonOutput, "json", false, "Write structured JSON output")
	pf.BoolVar(&noColor, "no-color", false, "Disable terminal colors (also honors NO_COLOR)")
	pf.StringVar(&flags.server, "server", "", "Backend URL (IRCORD_SERVER_URL; default http://127.0.0.1:8080)")
	pf.StringVar(&flags.tokenFile, "admin-token-file", "", "Private admin credential file (IRCORD_ADMIN_TOKEN_FILE)")
	pf.StringVar(&flags.config, "config", "", "Optional JSON client configuration (IRCORD_CLI_CONFIG)")
	pf.DurationVar(&flags.timeout, "timeout", 10*time.Second, "Administration request timeout (IRCORD_CLI_TIMEOUT)")
	root.Flags().BoolVar(&showVersion, "version", false, "Show local version information")
	_ = root.MarkPersistentFlagFilename("admin-token-file")
	_ = root.MarkPersistentFlagFilename("config", "json")
	root.AddCommand(&cobra.Command{
		Use: "status", GroupID: "admin", Short: "Inspect a running backend using the admin API", Args: cobra.NoArgs,
		Long:    "Authenticate with the operator admin token and inspect the backend. Configuration precedence: explicit flags, environment, optional config file, then defaults. This command never opens a database or starts IRC.",
		Example: "ircordctl --server https://chat.example.com status\nircordctl status --json\nircordctl --config ./admin.json status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := flags.client(cmd, getenv)
			if err != nil {
				return err
			}
			defer client.Close()
			status, err := client.Status(cmd.Context())
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
				if err := p.Encode(status); err != nil {
					return &exitError{1, errors.New("cannot write status output")}
				}
			} else {
				p.Heading("IRCord backend")
				p.Row("Status", string(status.Status))
				p.Row("Version", status.Version)
				p.Row("Commit", status.Commit)
				p.Row("Go", status.GoVersion)
				p.Row("Database", string(status.Database))
				p.Row("Started", status.StartedAt.Format(time.RFC3339))
				p.Row("Uptime", (time.Duration(status.UptimeSeconds) * time.Second).String())
				if status.Overview != nil {
					p.Heading("Backend overview")
					p.Row("Accounts", strconv.FormatInt(status.Overview.Accounts, 10))
					p.Row("Sessions", strconv.FormatInt(status.Overview.Sessions, 10))
					p.Row("Saved networks", strconv.FormatInt(status.Overview.Networks, 10))
					p.Row("IRC connections", strconv.FormatInt(status.Overview.IrcConnections, 10))
					p.Row("Stored messages", strconv.FormatInt(status.Overview.Messages, 10))
					p.Row("Database bytes", strconv.FormatInt(status.Overview.DatabaseBytes, 10))
				}
				if status.Ergo != nil {
					renderErgoStatus(p, *status.Ergo)
				}
				fmt.Fprintln(out)
			}
			if status.Status != "ok" || status.Database != "available" {
				return &exitError{1, errors.New("backend health is degraded")}
			}
			return nil
		},
	})
	root.AddCommand(&cobra.Command{Use: "version", GroupID: "local", Short: "Show local version information", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return version() }})
	completion := &cobra.Command{
		Use: "completion [bash|zsh|fish|powershell]", GroupID: "local", Short: "Generate shell completion", ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:    cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Example: "ircordctl completion zsh > _ircordctl",
		RunE: func(cmd *cobra.Command, args []string) error {
			var b bytes.Buffer
			var err error
			switch args[0] {
			case "bash":
				err = root.GenBashCompletionV2(&b, true)
			case "zsh":
				err = root.GenZshCompletion(&b)
			case "fish":
				err = root.GenFishCompletion(&b, true)
			case "powershell":
				err = root.GenPowerShellCompletionWithDesc(&b)
			}
			if err != nil {
				return &exitError{1, errors.New("cannot generate completion script")}
			}
			if printer().JSON {
				return printer().Encode(map[string]string{"shell": args[0], "script": b.String()})
			}
			_, err = io.Copy(out, &b)
			return err
		},
	}
	root.AddCommand(completion)
	root.AddCommand(remoteCommand("doctor", "Check backend and configured integrations", cobra.NoArgs, &flags, getenv, printer,
		func(ctx context.Context, c *adminclient.Client, _ []string) (contract.Doctor, error) {
			return c.Doctor(ctx)
		},
		func(p ui.Printer, v contract.Doctor) {
			p.Heading("Administration checks")
			for _, check := range v.Checks {
				p.Row(check.Name, string(check.Status)+" — "+check.Detail)
			}
		},
		func(v contract.Doctor) bool { return v.Status == "ok" }))
	if discoverErgo(ctx, args, getenv) {
		root.AddCommand(ergoCommands(&flags, getenv, printer))
	}
	return root, printer
}

// Cobra can reject arguments before flags are fully parsed. Resolve output modes
// from argv as well so those errors remain machine-readable and color-free.
func enabledFlag(args []string, name string) bool {
	enabled := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--"+name {
			enabled = true
		} else if value, ok := strings.CutPrefix(arg, "--"+name+"="); ok {
			enabled, _ = strconv.ParseBool(value)
		}
	}
	return enabled
}
