package msgindex

import (
	"net/url"
	"regexp"
	"strings"
)

var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// youtubeID groups URL variants in the link catalog. It does not fetch metadata
// or construct players; rendering is handled by the link-media resolver.
func youtubeID(u *url.URL) string {
	if u == nil || u.Scheme != "https" || u.User != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	var id string
	switch strings.ToLower(u.Hostname()) {
	case "youtu.be":
		if len(segments) == 1 {
			id = segments[0]
		}
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com", "www.youtube-nocookie.com":
		if len(segments) == 1 && segments[0] == "watch" {
			id = u.Query().Get("v")
		} else if len(segments) == 2 {
			switch segments[0] {
			case "embed", "live", "v", "shorts":
				id = segments[1]
			}
		}
	}
	if !youtubeIDPattern.MatchString(id) {
		return ""
	}
	return id
}
