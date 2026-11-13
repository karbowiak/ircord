package linkmedia

import (
	"net/url"
	"regexp"
	"strings"
)

var tiktokHandler = providerHandler{
	name:         "tiktok",
	hosts:        []string{"tiktok.com", "www.tiktok.com", "m.tiktok.com", "vm.tiktok.com", "vt.tiktok.com"},
	canonicalURL: tiktokURL,
	extractor:    "TikTok",
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
