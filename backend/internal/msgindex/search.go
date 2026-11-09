package msgindex

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"ircord/backend/internal/model"
)

// SearchKind is a persisted bit mask, independent of the link catalog's kinds.
func SearchKind(kind string) int {
	return map[string]int{"link": 1, "image": 2, "gif": 4, "video": 8, "file": 16}[kind]
}

// SearchKinds includes attachments and picker GIFs without decoding payloads in SQL.
func SearchKinds(msg model.Message) int {
	kinds := 0
	// Unlike the display catalog, search covers links after the first twenty.
	for _, raw := range candidate.FindAllString(OutsideCode(msg.Text), -1) {
		link, err := url.Parse(trimTrailing(raw))
		if err != nil || link.Hostname() == "" {
			continue
		}
		kinds |= SearchKind("link")
		switch kind(link) {
		case KindImage:
			kinds |= SearchKind("image")
		case KindGIF:
			kinds |= SearchKind("gif")
		case KindVideo, KindYouTube:
			kinds |= SearchKind("video")
		}
	}
	if msg.GIF != nil {
		kinds |= SearchKind("gif")
	}
	for _, a := range msg.AllAttachments() {
		switch contentType := strings.ToLower(strings.TrimSpace(strings.Split(a.ContentType, ";")[0])); {
		case contentType == "image/gif":
			kinds |= SearchKind("gif")
		case strings.HasPrefix(contentType, "image/"):
			kinds |= SearchKind("image")
		case strings.HasPrefix(contentType, "video/"):
			kinds |= SearchKind("video")
		default:
			kinds |= SearchKind("file")
		}
	}
	return kinds
}

// SearchTokens mirrors unicode61's letter/number tokens and diacritic folding
// for the in-memory search path. Punctuation is never interpreted as FTS syntax.
func SearchTokens(text string) []string {
	var folded strings.Builder
	latin := false
	for _, r := range strings.ToLower(text) {
		if unicode.Is(unicode.Mn, r) && latin {
			continue
		}
		latin = unicode.In(r, unicode.Latin)
		if !latin {
			folded.WriteRune(r)
			continue
		}
		// unicode61 removes diacritics from Latin letters, while keeping
		// precomposed letters in other scripts (for example Cyrillic й).
		for _, part := range norm.NFD.String(string(r)) {
			if !unicode.Is(unicode.Mn, part) {
				folded.WriteRune(part)
			}
		}
	}
	return strings.FieldsFunc(folded.String(), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.Is(unicode.Co, r)
	})
}

// SearchPhrase matches a consecutive phrase with a prefix on its last word.
func SearchPhrase(text, query string) bool {
	words, terms := SearchTokens(text), SearchTokens(query)
	if len(terms) == 0 {
		return false
	}
	for i := 0; i+len(terms) <= len(words); i++ {
		last := len(terms) - 1
		if slices.Equal(words[i:i+last], terms[:last]) && strings.HasPrefix(words[i+last], terms[last]) {
			return true
		}
	}
	return false
}

// MentionPattern preserves punctuation in IRC nicks and rejects nick substrings.
func MentionPattern(nick string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|[^\pL\pN_-])@?` + regexp.QuoteMeta(nick) + `($|[^\pL\pN_-])`)
}
