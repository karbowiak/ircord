package notify

import (
	"slices"
	"strings"
	"unicode"
)

// Name mentions (#209) need an "@": "hey @alice" highlights alice, "Alice is
// here" does not. Plain words still highlight through highlight words, so a
// user who wants the old behavior adds their own nick there. Only the
// account's nick on the message's network counts; display names and account
// names are not aliases. The browser applies the same rule
// (frontend/shared/mentions.ts); testdata/mentions.json lists the cases both follow.

// mentionOpeners may come right before the "@": text before it is a word,
// an address or a path ("bob@alice.example", "example.com/@alice").
const mentionOpeners = "([{<\"'“‘«*,;"

// NickMentioned reports whether text, with IRC formatting already removed,
// holds an explicit "@nick" mention. Nicks compare under the network's
// CASEMAPPING ("ascii", "rfc1459" when empty, "strict-rfc1459", anything else
// by Unicode lower case). The mention must be a whole nick: "@alice123",
// "@alice-" and "@alice.example" do not mention alice, and a mention inside a
// URL does not count.
func NickMentioned(text, nick, caseMapping string) bool {
	if nick == "" {
		return false
	}
	want := foldNick([]rune(nick), caseMapping)
	runes := []rune(text)
	for i, r := range runes {
		if r != '@' || i > 0 && !unicode.IsSpace(runes[i-1]) && !strings.ContainsRune(mentionOpeners, runes[i-1]) {
			continue
		}
		end := i + 1 + len(want)
		if end > len(runes) || !slices.Equal(foldNick(runes[i+1:end], caseMapping), want) {
			continue
		}
		if end < len(runes) && continuesNick(runes[end:]) || inURL(runes[:i]) {
			continue
		}
		return true
	}
	return false
}

// nickRune is a character a nick can hold: the RFC 1459 set, and letters,
// digits and marks of other scripts for networks that allow them.
func nickRune(r rune) bool {
	if r < 0x80 {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-[]\\`^_{|}", r)
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

// continuesNick reports whether rest, the text right after a candidate
// mention, makes it part of a longer nick, address or host name.
func continuesNick(rest []rune) bool {
	r := rest[0]
	if nickRune(r) || r == '@' {
		return true
	}
	return r == '.' && len(rest) > 1 && (unicode.IsLetter(rest[1]) || unicode.IsDigit(rest[1]))
}

// inURL reports whether the word that ends at before contains "://".
func inURL(before []rune) bool {
	start := len(before)
	for start > 0 && !unicode.IsSpace(before[start-1]) {
		start--
	}
	return strings.Contains(string(before[start:]), "://")
}

func foldNick(runes []rune, caseMapping string) []rune {
	out := make([]rune, len(runes))
	for i, r := range runes {
		out[i] = foldRune(r, caseMapping)
	}
	return out
}

func foldRune(r rune, caseMapping string) rune {
	m := strings.ToLower(caseMapping)
	if m != "" && m != "ascii" && m != "rfc1459" && m != "strict-rfc1459" {
		return unicode.ToLower(r)
	}
	switch {
	case r >= 'A' && r <= 'Z':
		return r + 'a' - 'A'
	case m == "ascii":
		return r
	case r == '[':
		return '{'
	case r == ']':
		return '}'
	case r == '\\':
		return '|'
	case r == '~' && m != "strict-rfc1459":
		return '^'
	}
	return r
}
