package linkmedia

import (
	"net/url"
	"regexp"
	"strings"

	"ircord/backend/internal/iframely"
)

var tiktokHandler = providerHandler{
	name:         "tiktok",
	hosts:        []string{"tiktok.com", "www.tiktok.com", "m.tiktok.com", "vm.tiktok.com", "vt.tiktok.com"},
	canonicalURL: tiktokURL,
	connectors:   func(options ResolverOptions) []Connector { return []Connector{options.Downloader} },
	extractor:    "TikTok",
	widget:       tiktokWidget,
}

var tiktokPath = regexp.MustCompile(`^/@[A-Za-z0-9_.]{1,64}/video/[0-9]{1,30}/?$`)
var shortTikTok = regexp.MustCompile(`^/[A-Za-z0-9]+/?$`)

func tiktokURL(u *url.URL) (string, error) {
	if u.Scheme != "https" || u.User != nil || u.RawPath != "" || u.Port() != "" && u.Port() != "443" || !socialQuery(u) {
		return "", ErrUnsupported
	}
	switch strings.ToLower(u.Hostname()) {
	case "tiktok.com", "www.tiktok.com", "m.tiktok.com":
		if !tiktokPath.MatchString(u.Path) {
			return "", ErrUnsupported
		}
	case "vm.tiktok.com", "vt.tiktok.com":
		if !shortTikTok.MatchString(u.Path) {
			return "", ErrUnsupported
		}
	default:
		return "", ErrUnsupported
	}
	if tiktokPath.MatchString(u.Path) {
		u.Host = "www.tiktok.com"
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	u.RawQuery = ""
	return u.String(), nil
}

// When direct CDN delivery fails and Iframely has no player, use TikTok's
// public iframe for a validated full video link. Short links need extraction.
func tiktokWidget(u *url.URL, embed iframely.Response) (Result, bool) {
	if embed.HTML != "" {
		return Result{}, false
	}
	raw := u.String()
	u.Scheme = "https"
	if _, err := tiktokURL(u); err != nil || !tiktokPath.MatchString(u.Path) {
		return Result{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	embed.HTML = `<iframe src="https://www.tiktok.com/player/v1/` + parts[2] + `?autoplay=0" style="width:100%;height:578px;border:0" allowfullscreen></iframe>`
	return Result{URL: raw, Provider: "tiktok", Title: "TikTok video", Embed: &embed}, true
}
