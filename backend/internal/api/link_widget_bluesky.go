package api

import (
	"net/url"
	"regexp"
	"strings"
)

var blueskyRecord = regexp.MustCompile(`^at://(did:[A-Za-z0-9:._%\-]+)/app\.bsky\.feed\.post/([A-Za-z0-9]+)$`)

func blueskyWidgetFrame(source *url.URL, markup scriptWidgetMarkup) string {
	if !strings.EqualFold(source.Hostname(), "bsky.app") || !markup.hasScript("/static/embed.js", "embed.bsky.app") {
		return ""
	}
	parts := strings.Split(strings.Trim(source.Path, "/"), "/")
	var record string
	for _, attributes := range markup.blockquotes {
		if value, ok := attributes["data-bluesky-uri"]; ok {
			record = value
		}
	}
	match := blueskyRecord.FindStringSubmatch(record)
	if len(match) == 3 && len(parts) == 4 && parts[0] == "profile" && parts[2] == "post" && parts[3] == match[2] {
		return "https://embed.bsky.app/embed/" + url.PathEscape(match[1]) + "/app.bsky.feed.post/" + match[2] + "?id=ircord"
	}
	return ""
}
