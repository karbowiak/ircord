// Package ui centralizes CLI output. A printer is invocation-local so tests and
// embedded command trees do not share mutable terminal settings.
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"
)

type Printer struct {
	Out, Err io.Writer
	JSON     bool
	Color    bool
}

func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func (p Printer) Accent(value string) string {
	if p.Color {
		return "\x1b[36m" + value + "\x1b[0m"
	}
	return value
}

func (p Printer) Heading(value string) {
	if !p.JSON {
		fmt.Fprintf(p.Out, "\n%s\n", p.Accent(value))
	}
}

func (p Printer) Row(label, value string) {
	if !p.JSON {
		fmt.Fprintf(p.Out, "  %-19s %s\n", label, Plain(value))
	}
}

// Terminal control sequences and bidi controls are never interpreted as output.
func Plain(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, value)
}

func (p Printer) Encode(value any) error {
	e := json.NewEncoder(p.Out)
	e.SetIndent("", "  ")
	return e.Encode(value)
}

func (p Printer) Error(message string, code int) {
	if p.JSON {
		_ = json.NewEncoder(p.Err).Encode(struct {
			Error string `json:"error"`
			Code  int    `json:"code"`
		}{message, code})
		return
	}
	label := "Error:"
	if p.Color && IsTerminal(p.Err) {
		label = "\x1b[31m" + label + "\x1b[0m"
	}
	fmt.Fprintln(p.Err, label, message)
}
