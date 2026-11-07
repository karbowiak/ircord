// Package cli exposes local backend commands. Remote administration belongs to
// the standalone cli/ module; this command tree can call backend services directly.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"ircord/backend/internal/buildinfo"
	"ircord/backend/internal/ui"
)

func Execute(ctx context.Context, args []string, out, errOut io.Writer, serve func(context.Context) error) int {
	if len(args) == 0 {
		args = []string{"serve"} // Preserve container/supervisor startup.
	}
	var jsonOutput, noColor, showVersion bool
	printer := func() ui.Printer {
		return ui.Printer{Out: out, Err: errOut, JSON: jsonOutput || enabledFlag(args, "json"),
			Color: !jsonOutput && !enabledFlag(args, "json") && !noColor && !enabledFlag(args, "no-color") && os.Getenv("NO_COLOR") == "" && ui.IsTerminal(out)}
	}
	info := buildinfo.Current()
	version := func() error {
		if printer().JSON {
			return printer().Encode(info)
		}
		printer().Heading("IRCord backend " + info.Version)
		printer().Row("Commit", info.Commit)
		printer().Row("Go", info.GoVersion)
		fmt.Fprintln(out)
		return nil
	}
	root := &cobra.Command{
		Use: "ircord", Short: "IRCord backend",
		Long:         "Run the IRCord backend using IRCORD_* environment settings.\nWith no arguments, IRCord starts the server. Use the separate ircordctl client for remote administration.",
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
	root.SetArgs(args)
	root.AddGroup(&cobra.Group{ID: "server", Title: "Server"}, &cobra.Group{ID: "local", Title: "Local tools"})
	root.SetHelpCommandGroupID("local")
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) { renderHelp(cmd, printer()) })
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Write structured JSON output/logs")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable terminal colors (also honors NO_COLOR)")
	root.Flags().BoolVar(&showVersion, "version", false, "Show local version information")
	code := 2
	root.AddCommand(&cobra.Command{Use: "serve", GroupID: "server", Short: "Start the backend and its IRC connections", Args: cobra.NoArgs,
		Example: "ircord serve\nircord serve --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code = 1
			if serve == nil {
				return errors.New("server startup is unavailable")
			}
			if printer().JSON {
				slog.SetDefault(slog.New(slog.NewJSONHandler(errOut, nil)))
			}
			return serve(cmd.Context())
		}})
	root.AddCommand(&cobra.Command{Use: "version", GroupID: "local", Short: "Show local version information", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return version() }})
	root.AddCommand(&cobra.Command{
		Use: "completion [bash|zsh|fish|powershell]", GroupID: "local", Short: "Generate shell completion",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"}, Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(_ *cobra.Command, args []string) error {
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
				return err
			}
			if printer().JSON {
				return printer().Encode(map[string]string{"shell": args[0], "script": b.String()})
			}
			_, err = io.Copy(out, &b)
			return err
		}})
	if err := root.ExecuteContext(ctx); err != nil {
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
