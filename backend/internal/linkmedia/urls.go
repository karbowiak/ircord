package linkmedia

import (
	"ircord/backend/internal/mediafetch"
	"regexp"
	"strings"
)

var messageURL = regexp.MustCompile(`https?://[^\s<>"'\x00-\x1f]+`)

// URLs finds link tokens in retained message text for request authorization.
// Code suppression is a presentation decision made by the client; this exact
// token check prevents a request from resolving an unrelated URL or substring.
func URLs(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range messageURL.FindAllString(text, -1) {
		raw = strings.TrimRight(raw, ".,!?;:")
		for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
			for strings.HasSuffix(raw, pair[1]) && strings.Count(raw, pair[1]) > strings.Count(raw, pair[0]) {
				raw = strings.TrimSuffix(raw, pair[1])
			}
		}
		if _, err := mediafetch.URL(raw); err != nil || seen[raw] {
			continue
		}
		seen[raw] = true
		out = append(out, raw)
	}
	return out
}
