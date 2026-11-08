package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"ircord/cli/internal/adminclient"
	"ircord/cli/internal/ui"
)

// Exit codes of accounts reset-password beyond the shared ones; the command's help text is
// the documented list.
const (
	exitResetPending = 1
	exitNotFound     = 5
	exitRefused      = 6
)

const maxPasswordInput = 4096

// terminalInput reports the descriptor of an interactive terminal behind r. Tests replace it.
var terminalInput = func(r io.Reader) (int, bool) {
	f, ok := r.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return 0, false
	}
	return int(f.Fd()), true
}

// readHidden reads one line without echo. Cancellation restores the terminal first, so an
// interrupted prompt never leaves echo switched off. Tests replace it.
var readHidden = func(ctx context.Context, fd int) ([]byte, error) {
	state, err := term.GetState(fd)
	if err != nil {
		return nil, err
	}
	type line struct {
		b   []byte
		err error
	}
	done := make(chan line, 1)
	go func() {
		b, err := term.ReadPassword(fd)
		done <- line{b, err}
	}()
	select {
	case v := <-done:
		return v.b, v.err
	case <-ctx.Done():
		_ = term.Restore(fd, state)
		return nil, ctx.Err()
	}
}

// trimLine removes one trailing line ending, as written by echo or an editor. Other
// characters are kept so the backend applies the same rules as Account settings.
func trimLine(b []byte) []byte {
	b = bytes.TrimSuffix(b, []byte("\n"))
	return bytes.TrimSuffix(b, []byte("\r"))
}

func readBoundedPassword(r io.Reader, source string) (string, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxPasswordInput+1))
	if err != nil {
		return "", &exitError{2, fmt.Errorf("cannot read the new password from %s", source)}
	}
	if len(b) > maxPasswordInput {
		return "", &exitError{2, fmt.Errorf("the new password from %s is longer than %d bytes", source, maxPasswordInput)}
	}
	b = trimLine(b)
	if len(b) == 0 {
		return "", &exitError{2, fmt.Errorf("the new password from %s is empty", source)}
	}
	return string(b), nil
}

func newPassword(ctx context.Context, cmd *cobra.Command, username string, fromStdin bool, file string) (string, error) {
	switch {
	case fromStdin && file != "":
		return "", &exitError{2, errors.New("use either --password-stdin or --password-file, not both")}
	case fromStdin:
		return readBoundedPassword(cmd.InOrStdin(), "standard input")
	case file != "":
		f, err := os.Open(file)
		if err != nil {
			return "", &exitError{2, errors.New("cannot open the password file")}
		}
		defer f.Close()
		if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
			return "", &exitError{2, errors.New("the password file must be a regular file")}
		}
		return readBoundedPassword(f, "the password file")
	}
	fd, ok := terminalInput(cmd.InOrStdin())
	if !ok {
		return "", &exitError{2, errors.New("no terminal for a hidden password prompt; pass the password with --password-stdin or --password-file")}
	}
	prompt := cmd.ErrOrStderr()
	var entries [2]string
	for i, label := range []string{"New password for " + ui.Plain(username) + ": ", "Repeat the new password: "} {
		fmt.Fprint(prompt, label)
		b, err := readHidden(ctx, fd)
		fmt.Fprintln(prompt)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", &exitError{2, errors.New("cannot read the new password from the terminal")}
		}
		entries[i] = string(trimLine(b))
	}
	if entries[0] == "" {
		return "", &exitError{2, errors.New("the new password is empty")}
	}
	if entries[0] != entries[1] {
		return "", &exitError{2, errors.New("the passwords don’t match; nothing was changed")}
	}
	return entries[0], nil
}

func resetExitCode(err error) int {
	var apiErr *adminclient.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case 401, 403:
			return 3
		case 404:
			// Only the backend's own problem answer means "no such account". A bare 404 is
			// a backend without this route, which is a backend problem.
			if apiErr.Detail != "" {
				return exitNotFound
			}
		case 501:
			return exitNotFound
		case 400, 422:
			return 2
		case 409:
			return exitRefused
		}
	}
	return 4
}

func accountCommands(flags *remoteFlags, getenv func(string) string, printer func() ui.Printer) *cobra.Command {
	root := &cobra.Command{Use: "accounts", Short: "Administer IRCord accounts", GroupID: "admin"}
	var fromStdin bool
	var file string
	reset := &cobra.Command{
		Use:   "reset-password <username>",
		Short: "Set a new password for an account",
		Long: `Set a new password for an existing IRCord account without its current password.
The backend applies the same rules as Account settings, changes the account on the
associated IRC server, updates the home network's saved IRC login when it still signs
in as this account (reconnecting it if connected), and signs out every session.

The new password is read from a hidden prompt, or from --password-stdin or
--password-file (one trailing line ending is removed). It is never taken from
command arguments and never printed.

Exit codes: 0 completed; 1 pending (saved; the backend keeps retrying, and running the
command again with the same password retries now); 2 invalid input or password;
3 admin authentication rejected; 4 backend unreachable, failed, timed out or without
this command; 5 no such account, or the backend has no IRCord accounts; 6 the associated
IRC server refused the change outright (for example, it has no account of that name), so
nothing was changed; 130 cancelled.`,
		Example: "ircordctl accounts reset-password alice\nprintf '%s\\n' \"$NEW_PASSWORD\" | ircordctl accounts reset-password alice --password-stdin --json\nircordctl accounts reset-password alice --password-file ./new-password",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := flags.client(cmd, getenv)
			if err != nil {
				return err
			}
			defer client.Close()
			password, err := newPassword(cmd.Context(), cmd, args[0], fromStdin, file)
			if err != nil {
				return err
			}
			result, err := client.ResetPassword(cmd.Context(), args[0], password)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					err = fmt.Errorf("password reset timed out; the backend may still finish it, so run the command again with the same password to confirm: %w", err)
				}
				return &exitError{resetExitCode(err), err}
			}
			p := printer()
			if p.JSON {
				if err := p.Encode(result); err != nil {
					return &exitError{1, errors.New("cannot write administration output")}
				}
			} else {
				p.Heading("Password reset · " + result.Username)
				p.Row("Status", string(result.Status))
				p.Row("Sessions signed out", strconv.FormatInt(result.SessionsRevoked, 10))
				switch {
				case !result.Associated:
					p.Row("IRC server", "not associated")
				case result.Status == "completed":
					p.Row("IRC server", "updated")
				default:
					p.Row("IRC server", "pending")
				}
				if result.Detail != nil {
					p.Row("Detail", *result.Detail)
				}
				fmt.Fprintln(p.Out)
			}
			if result.Status != "completed" {
				return &exitError{exitResetPending, errors.New("password reset is pending; the backend keeps retrying, and sign-in stays blocked until it finishes")}
			}
			return nil
		},
	}
	reset.GroupID = ""
	reset.Flags().BoolVar(&fromStdin, "password-stdin", false, "Read the new password from standard input")
	reset.Flags().StringVar(&file, "password-file", "", "Read the new password from a private file")
	_ = reset.MarkFlagFilename("password-file")
	root.AddCommand(reset)
	return root
}
