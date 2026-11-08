// Package msgindex derives catalog entries (images now, links later) from stored
// messages. Live delivery, history fetches and the backfill of retained history
// all use the same rules, so an entry never depends on a client being open.
package msgindex

import (
	"regexp"
	"strings"
)

// LinkURL finds the HTTPS links of message text, as server link previews do.
var LinkURL = regexp.MustCompile(`https://[^\s<>\x00-\x20]+`)

var fenceHint = regexp.MustCompile(`^([\w+#.-]{1,32})?[ \t]*\n`)

// CodeSegment is a fenced ``` block or the ordinary text between blocks.
type CodeSegment struct {
	Code bool
	Raw  string
}

// SplitCodeBlocks is the Go port of the frontend's splitCodeBlocks
// (frontend/shared/utils/irc.ts); both must classify the same text as code.
// A fence opens at a line start, may name a language on its first line and
// closes at the next ```. Hint-only, blank and unterminated fences stay text.
func SplitCodeBlocks(text string) []CodeSegment {
	var segments []CodeSegment
	pending, at := 0, 0
	for {
		i := strings.Index(text[at:], "```")
		if i < 0 {
			break
		}
		at += i
		if at > 0 && text[at-1] != '\n' {
			at += 3
			continue
		}
		end := strings.Index(text[at+3:], "```")
		if end < 0 {
			break
		}
		closing := at + 3 + end
		code := text[at+3 : closing]
		if hint := fenceHint.FindStringIndex(code); hint != nil {
			code = code[hint[1]:]
		}
		if strings.TrimSpace(strings.TrimSuffix(code, "\n")) == "" {
			at += 3
			continue
		}
		if at > pending {
			segments = append(segments, CodeSegment{Raw: text[pending:at]})
		}
		segments = append(segments, CodeSegment{Code: true, Raw: text[at : closing+3]})
		pending, at = closing+3, closing+3
	}
	if pending < len(text) {
		segments = append(segments, CodeSegment{Raw: text[pending:]})
	}
	return segments
}

// OutsideCode blanks code so links inside it are not previewed, like the
// frontend's textOutsideCode: fenced blocks first, then IRC monospace runs
// inside each text segment. Monospace toggles at \x11 and is reset by \x0f.
func OutsideCode(text string) string {
	var out strings.Builder
	for _, segment := range SplitCodeBlocks(text) {
		if segment.Code {
			out.WriteByte(' ')
			continue
		}
		mono := false
		for i := 0; i < len(segment.Raw); i++ {
			switch b := segment.Raw[i]; {
			case b == '\x11':
				mono = !mono
				out.WriteByte(' ')
			case b == '\x0f':
				mono = false
				out.WriteByte(' ')
			case !mono:
				out.WriteByte(b)
			}
		}
	}
	return out.String()
}
