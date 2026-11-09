package msgindex

import (
	"net/url"
	"regexp"
	"strings"

	"ircord/backend/internal/model"
)

// Version changes restart the bounded backfill of retained history.
// Version 4 also builds the text search index and attachment content flags.
const Version = 4

const (
	KindImage = "image"
	KindGIF   = "gif"
)

var imagePath = regexp.MustCompile(`(?i)\.(png|jpe?g|webp|avif|heic)$`)
var gifPath = regexp.MustCompile(`(?i)\.(gif|apng)$`)
var animatedPath = regexp.MustCompile(`(?i)\.(webp|avif)$`)

// animatedHosts serve animated WebP and AVIF as their GIF formats; gifDomains
// cover every subdomain. Both match the frontend's lists.
var animatedHosts = []string{"media.tenor.com", "media.giphy.com", "i.giphy.com", "static.klipy.com", "cdn.7tv.app"}
var gifDomains = []string{"tenor.com", "giphy.com"}

func animatedHost(host string) bool {
	host = strings.ToLower(host)
	for _, h := range animatedHosts {
		if host == h {
			return true
		}
	}
	for _, d := range gifDomains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// ImageKind classifies a URL by its path as the timeline does: KindGIF for a
// .gif or .apng (or an animated format on a GIF host), KindImage for other
// images, and "" for anything else.
func ImageKind(u *url.URL) string {
	switch {
	case gifHost(u.Hostname()):
		return KindGIF
	case gifPath.MatchString(u.Path), animatedPath.MatchString(u.Path) && animatedHost(u.Hostname()):
		return KindGIF
	case imagePath.MatchString(u.Path):
		return KindImage
	}
	return ""
}

func gifHost(host string) bool {
	host = strings.ToLower(host)
	for _, domain := range gifDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// Indexed reports whether a message kind can carry catalogued content.
func Indexed(msg model.Message) bool {
	return !msg.Deleted && (msg.Kind == "message" || msg.Kind == "action" || msg.Kind == "notice")
}
