package msgindex

import (
	"net"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/idna"
)

// Text links get a catalog identity, grouping posts of the same resource.
const (
	KindLink    = "link"
	KindVideo   = "video"
	KindAudio   = "audio"
	KindYouTube = "youtube"
)

// MaxLinksPerMessage bounds how many links one message adds to the catalog.
const MaxLinksPerMessage = 20

// MaxLinkLength is the longest URL, and the longest grouping identity, the
// catalog keeps; longer ones stay plain text.
const MaxLinkLength = 2048

// Link is one link a message names.
type Link struct {
	// URL is the link as posted, after trimming sentence punctuation.
	URL string
	// Identity groups posts of the same resource: the URL with its scheme and
	// host lowercased, the default port and the fragment dropped, and the path
	// and query kept exactly; a YouTube link is its video.
	Identity string
	Host     string
	Kind     string
}

// candidate finds links like the timeline: a lowercase http:// or https:// up to whitespace. Control
// characters (IRC formatting) and markup delimiters also end it.
var candidate = regexp.MustCompile(`https?://[^\s\x00-\x1f\x7f<>"]+`)

// Links returns the links of a message text outside code, in order, at most
// MaxLinksPerMessage. Links with credentials, without a host or longer than
// MaxLinkLength are left out; the same URL posted twice in one message counts once.
func Links(text string) []Link {
	out := []Link{}
	seen := map[string]bool{}
	for _, raw := range candidate.FindAllString(OutsideCode(text), -1) {
		raw = trimTrailing(raw)
		if len(raw) > MaxLinkLength {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || u.Hostname() == "" {
			continue
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "http" && scheme != "https" {
			continue
		}
		link := Link{URL: raw, Host: NormalizeHost(u.Hostname()), Kind: kind(u)}
		link.Identity = identity(u, link.Kind)
		// Percent-encoding can make an identity longer than its URL; page cursors
		// carry identities, so they are bounded too.
		if len(link.Identity) > MaxLinkLength {
			continue
		}
		if seen[raw] {
			continue
		}
		seen[raw] = true
		out = append(out, link)
		if len(out) == MaxLinksPerMessage {
			break
		}
	}
	return out
}

// trimTrailing drops sentence punctuation after a link ("see https://x.test.")
// and closing brackets the link did not open ("(https://x.test)"), so
// https://en.wikipedia.org/wiki/Go_(programming_language) keeps its own.
func trimTrailing(raw string) string {
	for raw != "" {
		last := raw[len(raw)-1]
		switch last {
		case '.', ',', ';', ':', '!', '?', '\'', '*', '_', '~', '`':
			raw = raw[:len(raw)-1]
			continue
		case ')', ']', '}':
			open := map[byte]byte{')': '(', ']': '[', '}': '{'}[last]
			if strings.Count(raw, string(open)) < strings.Count(raw, string(last)) {
				raw = raw[:len(raw)-1]
				continue
			}
		}
		return raw
	}
	return raw
}

func kind(u *url.URL) string {
	if youtubeID(u) != "" {
		return KindYouTube
	}
	if k := ImageKind(u); k != "" {
		return k
	}
	switch {
	case videoPath.MatchString(u.Path):
		return KindVideo
	case audioPath.MatchString(u.Path):
		return KindAudio
	}
	return KindLink
}

var (
	videoPath = regexp.MustCompile(`(?i)\.(mp4|webm|mov|m4v)$`)
	audioPath = regexp.MustCompile(`(?i)\.(mp3|m4a|aac|ogg|oga|opus|wav|flac)$`)
)

// identity is deliberately conservative: two links group only when they name
// the same resource. Query strings (including signed ones) and path case are
// kept; only parts that never select a different resource are normalized.
func identity(u *url.URL, kind string) string {
	if kind == KindYouTube {
		return "youtube:" + youtubeID(u)
	}
	scheme := strings.ToLower(u.Scheme)
	host := NormalizeHost(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := u.Port(); port != "" && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		host = net.JoinHostPort(strings.Trim(host, "[]"), port)
	}
	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	out := scheme + "://" + host + p
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	return out
}

// NormalizeHost is the spelling hosts are grouped and filtered by: lowercase,
// without a trailing dot, and in ASCII (punycode) form, so bücher.example and
// xn--bcher-kva.example are one host. IP addresses take their canonical form.
// A name IDNA refuses keeps its lowercase form.
func NormalizeHost(host string) string {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	if ascii, err := idna.Lookup.ToASCII(host); err == nil && ascii != "" {
		return ascii
	}
	return host
}

// FilterHost reads a host filter: a host name (Unicode or punycode), an IP
// address, or an IPv6 address in brackets. It reports false for anything else.
func FilterHost(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		raw = raw[1 : len(raw)-1]
	}
	if raw == "" || len(raw) > 253 {
		return "", false
	}
	if ip := net.ParseIP(raw); ip != nil {
		return ip.String(), true
	}
	ascii, err := idna.Lookup.ToASCII(strings.TrimSuffix(strings.ToLower(raw), "."))
	if err != nil || ascii == "" {
		return "", false
	}
	return ascii, true
}
