package api

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
)

func xWidgetFrame(source *url.URL, markup scriptWidgetMarkup) string {
	switch strings.ToLower(source.Hostname()) {
	case "x.com", "www.x.com", "twitter.com", "www.twitter.com":
	default:
		return ""
	}
	if !markup.hasScript("/widgets.js", "platform.twitter.com", "platform.x.com") {
		return ""
	}
	quote := false
	for _, attributes := range markup.blockquotes {
		if slices.Contains(strings.Fields(attributes["class"]), "twitter-tweet") {
			quote = true
			break
		}
	}
	parts := strings.Split(strings.Trim(source.Path, "/"), "/")
	var tweet string
	if len(parts) == 3 && (parts[1] == "status" || parts[1] == "statuses") {
		tweet = parts[2]
	} else if len(parts) == 4 && parts[0] == "i" && parts[1] == "web" && parts[2] == "status" {
		tweet = parts[3]
	}
	if id, err := strconv.ParseUint(tweet, 10, 64); err == nil && id > 0 && quote {
		return "https://platform.twitter.com/embed/Tweet.html?" + url.Values{"id": {tweet}, "dnt": {"true"}}.Encode()
	}
	return ""
}
