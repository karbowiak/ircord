package linkmedia

// Provider URL patterns and player conventions adapted from Iframely's MIT
// plugins/domains at 962bf9287f2ab5880ff96d4ac796ebb62217be57.
// Copyright (c) 2013-2022 Itteco Software Corp. See iframely-providers.LICENSE.txt.
// This registry executes no upstream code and grants no authority to scripts,
// provider dimensions, arbitrary iframe origins, or postMessage measurements.

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	parse "golang.org/x/net/html"
	"ircord/backend/internal/mediafetch"
)

type publicProvider struct {
	name, source, frame string
	height              int
	ratio               float64
}

// Player geometry is mirrored by the static frontend embedProvider adapters.
var publicProviders = []publicProvider{
	{"dbdiagram", `^dbdiagram\.io$`, `^dbdiagram\.io/embed/[A-Fa-f0-9]{24}$`, 270, 16.0 / 9},
	{"slides", `^slides\.com$`, `^slides\.com/[A-Za-z0-9_-]+/[A-Za-z0-9_-]+/embed$`, 350, 48.0 / 35},
	{"quizlet", `^quizlet\.com$`, `^quizlet\.com/[0-9]+/(?:flashcards|match|learn|spell|test)/embed$`, 500, 0},
	{"openstreetmap", `^(?:www\.)?openstreetmap\.org$`, `^www\.openstreetmap\.org/export/embed\.html$`, 360, 4.0 / 3},
	{"yandex-maps", `^yandex\.(?:ru|com)$`, `^(?:yandex\.(?:ru|com)/map-widget/v1/(?:-/[A-Za-z0-9_~.-]+)?|api-maps\.yandex\.(?:ru|com)/frame/v1/-/[A-Za-z0-9_~.-]+)$`, 360, 4.0 / 3},
	{"padlet", `^padlet\.com$`, `^padlet\.com/embed/[a-z0-9]{12,32}$`, 608, 0},
	{"google-docs", `^(?:docs|drive)\.google\.com$`, `^(?:docs|drive)\.google\.com/(?:document|spreadsheets|presentation|forms|drawings|file)/d/(?:e/)?[A-Za-z0-9_-]+/(?:preview|embed|pub|pubhtml|htmlview|mobilebasic|viewform)$`, 640, 3.0 / 4},
	{"google-calendar", `^(?:www|calendar)\.google\.com$`, `^calendar\.google\.com/calendar/embed$`, 360, 4.0 / 3},
	{"google-maps", `^(?:www\.)?google\.com$`, `^www\.google\.com/maps/(?:d/)?embed$`, 360, 4.0 / 3},
	{"youtube", `^(?:www\.|m\.)?(?:youtube\.com|youtube-nocookie\.com)$`, `^(?:www\.)?youtube(?:-nocookie)?\.com/embed/[A-Za-z0-9_-]+$`, 270, 16.0 / 9},
	{"youtube", `^youtu\.be$`, `^(?:www\.)?youtube(?:-nocookie)?\.com/embed/[A-Za-z0-9_-]+$`, 270, 16.0 / 9},
	{"vimeo", `^(?:www\.|player\.)?vimeo\.com$`, `^player\.vimeo\.com/video/[0-9]+$`, 270, 16.0 / 9},
	{"spotify", `^(?:open|play)\.spotify\.com$`, `^open\.spotify\.com/embed/(?:track|album|playlist|artist|episode|show)/[A-Za-z0-9_-]+$`, 352, 0},
	{"soundcloud", `^(?:(?:www|m|w)\.)?soundcloud\.com$`, `^w\.soundcloud\.com/player/?$`, 166, 0},
	{"mixcloud", `^(?:www\.)?mixcloud\.com$`, `^www\.mixcloud\.com/widget/iframe/?$`, 120, 0},
	{"bandcamp", `^[a-z0-9-]+\.bandcamp\.com$`, `^bandcamp\.com/EmbeddedPlayer/(?:[a-z]+=[A-Za-z0-9_-]+/)+$`, 120, 0},
	{"dailymotion", `^(?:www\.|geo\.)?dailymotion\.com$`, `^(?:www\.dailymotion\.com/embed/video/[A-Za-z0-9_-]+|geo\.dailymotion\.com/player/[A-Za-z0-9_-]+\.html)$`, 270, 16.0 / 9},
	{"dailymotion", `^dai\.ly$`, `^(?:www\.dailymotion\.com/embed/video/[A-Za-z0-9_-]+|geo\.dailymotion\.com/player/[A-Za-z0-9_-]+\.html)$`, 270, 16.0 / 9},
	{"loom", `^(?:www\.)?loom\.com$`, `^www\.loom\.com/embed/[A-Za-z0-9_-]+$`, 270, 16.0 / 9},
	{"wistia", `^[a-z0-9-]+\.wistia\.com$`, `^fast\.wistia\.(?:net|com)/embed/iframe/[A-Za-z0-9_-]+$`, 270, 16.0 / 9},
	{"ted", `^(?:www\.|embed\.)?ted\.com$`, `^embed\.ted\.com/talks/(?:lang/[A-Za-z-]+/)?[A-Za-z0-9_-]+$`, 270, 16.0 / 9},
	{"twitch", `^(?:(?:www|go|clips)\.)?twitch\.tv$`, `^(?:player\.twitch\.tv/|clips\.twitch\.tv/embed)$`, 270, 16.0 / 9},
	{"applemusic", `^(?:(?:geo|embed)\.)?(?:music|itunes)\.apple\.com$`, `^embed\.music\.apple\.com/[a-zA-Z]{2}/(?:album|song|playlist|music-video)/[^?#]+$`, 450, 0},
	{"codepen", `^codepen\.io$`, `^codepen\.io/[A-Za-z0-9_-]+/embed/(?:preview/)?[A-Za-z0-9_-]+$`, 400, 0},
	{"jsfiddle", `^(?:www\.)?jsfiddle\.net$`, `^jsfiddle\.net/(?:[A-Za-z0-9_-]+/)?[A-Za-z0-9_-]+/(?:[0-9]+/)?embedded/(?:[A-Za-z,]+/)?$`, 400, 0},
	{"livecodes", `^(?:v[0-9]+\.)?livecodes\.io$`, `^(?:v[0-9]+\.)?livecodes\.io/$`, 400, 0},
	{"speakerdeck", `^speakerdeck\.com$`, `^speakerdeck\.com/player/[A-Za-z0-9_-]+$`, 360, 4.0 / 3},
	{"slideshare", `^(?:www\.)?slideshare\.net$`, `^www\.slideshare\.net/slideshow/embed_code/key/[A-Za-z0-9_-]+$`, 360, 4.0 / 3},
	{"deezer", `^(?:www\.)?deezer\.com$`, `^widget\.deezer\.com/widget/(?:auto|dark|light)/(?:track|album|playlist|artist)/[0-9]+$`, 300, 0},
	{"art19", `^(?:www\.)?art19\.com$`, `^art19\.com/shows/[A-Za-z0-9_-]+/(?:episodes/[A-Za-z0-9_-]+/)?embed$`, 200, 0},
	{"libsyn", `^[a-z0-9.-]+\.libsyn(?:pro)?\.com$`, `^(?:html5-)?play(?:er)?\.libsyn\.com/embed/episode/id/[0-9]+(?:/[A-Za-z0-9_/-]+)?/?$`, 96, 0},
	{"simplecast", `^(?:[a-z0-9_-]+\.)?simplecast\.com$`, `^(?:player|embed)\.simplecast\.com/[A-Za-z0-9_-]+/?$`, 200, 0},
	{"geogebra", `^(?:www\.)?geogebra\.org$`, `^www\.geogebra\.org/material/iframe/id/[A-Za-z0-9]+$`, 360, 4.0 / 3},
	{"documentcloud", `^(?:www\.|embed\.)?documentcloud\.org$`, `^embed\.documentcloud\.org/documents/[0-9]+(?:-[^/]+)?/?$`, 640, 3.0 / 4},
	{"prezi", `^(?:www\.)?prezi\.com$`, `^prezi\.com/embed/[A-Za-z0-9_-]+/?$`, 400, 11.0 / 8},
	{"scribd", `^(?:www|[a-z]{2})\.scribd\.com$`, `^www\.scribd\.com/embeds/[0-9]+/content$`, 640, 3.0 / 4},
}

type compiledPublicProvider struct {
	publicProvider
	sourcePattern, framePattern *regexp.Regexp
}

var compiledPublicProviders = func() []compiledPublicProvider {
	all := make([]compiledPublicProvider, 0, len(publicProviders))
	for _, p := range publicProviders {
		all = append(all, compiledPublicProvider{p, regexp.MustCompile(p.source), regexp.MustCompile(p.frame)})
	}
	return all
}()

func publicProviderFor(u *url.URL) *compiledPublicProvider {
	if u == nil || u.User != nil || u.Scheme != "https" && u.Scheme != "http" || u.Port() != "" && u.Port() != "443" || u.RawPath != "" || strings.Contains(u.Path, "\\") || !publicProviderQuery(u, 0) {
		return nil
	}
	for i := range compiledPublicProviders {
		p := &compiledPublicProviders[i]
		if p.sourcePattern.MatchString(strings.ToLower(u.Hostname())) && providerSourcePath(p.name, u) {
			return p
		}
	}
	return nil
}

var providerID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)
var decimalID = regexp.MustCompile(`^[0-9]{1,30}$`)
var applePlaylistID = regexp.MustCompile(`^pl\.[A-Za-z0-9._-]{1,190}$`)
var appleLocale = regexp.MustCompile(`^[A-Za-z]{2}$`)
var dailymotionVideoID = regexp.MustCompile(`^[A-Za-z0-9]{1,30}$`)

// Provider players never inherit private access keys or nested private URLs.
// Unknown harmless options are discarded when each player's query is rebuilt.
func publicProviderQuery(u *url.URL, depth int) bool {
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || depth > 2 {
		return false
	}
	for key, values := range query {
		switch strings.ToLower(key) {
		case "token", "secret_token", "access_token", "auth", "authorization", "password", "passwd", "secret", "api_key", "apikey", "key", "h", "signature", "sig":
			return false
		}
		for _, value := range values {
			// A Yandex share URL can include this non-network place identifier.
			// It is discarded; only validated ll/z values reach the map widget.
			if (u.Hostname() == "yandex.com" || u.Hostname() == "yandex.ru") && key == "poi[uri]" && yandexPlaceURI.MatchString(value) {
				continue
			}
			if strings.HasPrefix(value, "//") {
				value = "https:" + value
			}
			if strings.Contains(value, "://") {
				nested, err := mediafetch.URL(value)
				if err != nil || fileCapabilityURL(value) || !publicProviderQuery(nested, depth+1) {
					return false
				}
			}
		}
	}
	return true
}

func providerSourcePath(name string, u *url.URL) bool {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch name {
	case "dbdiagram", "slides", "quizlet", "openstreetmap", "yandex-maps", "padlet":
		_, ok := appFrameRoute(name, u)
		return ok
	case "google-docs":
		_, ok := googleDocumentURL(u)
		return ok
	case "google-calendar":
		_, ok := googleCalendarQuery(u)
		return ok
	case "google-maps":
		_, _, ok := googleMapQuery(u)
		return ok
	case "youtube":
		return u.Hostname() == "youtu.be" && len(parts) == 1 && providerID.MatchString(parts[0]) || len(parts) == 1 && parts[0] == "watch" && providerID.MatchString(u.Query().Get("v")) || len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") && providerID.MatchString(parts[1])
	case "vimeo":
		return len(parts) == 1 && decimalID.MatchString(parts[0]) || len(parts) == 2 && parts[0] == "video" && decimalID.MatchString(parts[1]) || len(parts) >= 3 && (parts[0] == "channels" || parts[0] == "groups") && decimalID.MatchString(parts[len(parts)-1])
	case "spotify":
		if len(parts) > 0 && strings.HasPrefix(parts[0], "intl-") {
			parts = parts[1:]
		}
		return len(parts) == 2 && strings.Contains("|track|album|playlist|artist|episode|show|", "|"+parts[0]+"|") && providerID.MatchString(parts[1])
	case "soundcloud":
		if u.Hostname() == "w.soundcloud.com" {
			return len(parts) == 1 && parts[0] == "player" && publicSoundCloudTarget(u.Query().Get("url"))
		}
		return len(parts) == 2 && providerID.MatchString(parts[0]) && providerID.MatchString(parts[1]) ||
			len(parts) == 3 && providerID.MatchString(parts[0]) && parts[1] == "sets" && providerID.MatchString(parts[2])
	case "mixcloud":
		return len(parts) == 2 && providerID.MatchString(parts[0]) && providerID.MatchString(parts[1])
	case "bandcamp":
		return len(parts) == 2 && (parts[0] == "track" || parts[0] == "album") && providerID.MatchString(parts[1])
	case "loom":
		return len(parts) == 2 && (parts[0] == "share" || parts[0] == "embed") && providerID.MatchString(parts[1])
	case "wistia":
		return len(parts) == 2 && parts[0] == "medias" && providerID.MatchString(parts[1])
	case "ted":
		return len(parts) == 2 && (parts[0] == "talks" || parts[0] == "dubbing") && providerID.MatchString(parts[1])
	case "twitch":
		return len(parts) == 1 && providerID.MatchString(parts[0]) || len(parts) == 2 && (parts[0] == "videos" || parts[0] == "video") && decimalID.MatchString(parts[1]) || len(parts) == 3 && providerID.MatchString(parts[0]) && (parts[1] == "clip" || parts[1] == "v") && providerID.MatchString(parts[2])
	case "applemusic":
		_, _, ok := appleMusicContent(u)
		return ok && (!u.Query().Has("i") || decimalID.MatchString(u.Query().Get("i")))
	case "codepen":
		return len(parts) == 3 && providerID.MatchString(parts[0]) && (parts[1] == "pen" || parts[1] == "details" || parts[1] == "full") && providerID.MatchString(parts[2])
	case "jsfiddle":
		return len(parts) >= 1 && len(parts) <= 3 && providerID.MatchString(parts[0]) && (len(parts) == 1 || providerID.MatchString(parts[1])) && (len(parts) < 3 || decimalID.MatchString(parts[2]))
	case "livecodes":
		return u.Path == "" || u.Path == "/"
	case "speakerdeck":
		return len(parts) == 2 && providerID.MatchString(parts[0]) && providerID.MatchString(parts[1])
	case "slideshare":
		return len(parts) >= 2 && parts[0] != "secret"
	case "dailymotion":
		shape := u.Hostname() == "dai.ly" && len(parts) == 1 && providerID.MatchString(parts[0]) ||
			len(parts) == 2 && parts[0] == "video" && providerID.MatchString(parts[1]) ||
			len(parts) == 3 && parts[0] == "embed" && parts[1] == "video" && providerID.MatchString(parts[2]) ||
			len(parts) == 2 && parts[0] == "player" && strings.HasSuffix(parts[1], ".html") && providerID.MatchString(strings.TrimSuffix(parts[1], ".html"))
		return shape && dailymotionVideoID.MatchString(dailymotionContentID(u))
	case "deezer":
		if len(parts) == 3 && len(parts[0]) == 2 {
			parts = parts[1:]
		}
		return len(parts) == 2 && strings.Contains("|track|album|playlist|artist|", "|"+parts[0]+"|") && decimalID.MatchString(parts[1])
	case "art19":
		return len(parts) == 2 && parts[0] == "shows" && providerID.MatchString(parts[1]) || len(parts) == 4 && parts[0] == "shows" && parts[2] == "episodes" && providerID.MatchString(parts[1]) && providerID.MatchString(parts[3])
	case "libsyn":
		return len(parts) >= 1 && parts[0] != "" && !strings.HasPrefix(u.Path, "/embed/")
	case "simplecast":
		return len(parts) == 2 && (parts[0] == "episodes" || parts[0] == "s") && providerID.MatchString(parts[1])
	case "geogebra":
		return len(parts) == 2 && (parts[0] == "m" || parts[0] == "classic" || parts[0] == "graphing") && providerID.MatchString(parts[1]) || len(parts) == 2 && parts[0] == "student" && strings.HasPrefix(parts[1], "m") && providerID.MatchString(parts[1])
	case "documentcloud":
		id, _, _ := strings.Cut(parts[len(parts)-1], "-")
		return len(parts) == 2 && (parts[0] == "documents" || parts[0] == "document") && decimalID.MatchString(id)
	case "prezi":
		return len(parts) == 2 && providerID.MatchString(parts[0]) && parts[0] != "p" && parts[0] != "view" && parts[0] != "embed" && parts[0] != "v"
	case "scribd":
		return len(parts) >= 2 && strings.Contains("|doc|document|embeds|presentation|fullscreen|", "|"+parts[0]+"|") && decimalID.MatchString(parts[1])
	}
	return false
}

func publicSoundCloudTarget(raw string) bool {
	u, err := mediafetch.URL(raw)
	if err != nil || u.Scheme != "https" || u.Port() != "" && u.Port() != "443" || u.RawPath != "" || u.RawQuery != "" {
		return false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if u.Hostname() == "api.soundcloud.com" || u.Hostname() == "api-v2.soundcloud.com" {
		return len(parts) == 2 && (parts[0] == "tracks" || parts[0] == "playlists") && decimalID.MatchString(parts[1])
	}
	return u.Hostname() == "soundcloud.com" && providerSourcePath("soundcloud", u)
}

func appleMusicContent(u *url.URL) (kind, id string, ok bool) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || len(parts) > 4 || !appleLocale.MatchString(parts[0]) {
		return
	}
	kind, id = parts[1], strings.TrimPrefix(parts[len(parts)-1], "id")
	switch kind {
	case "album", "song", "music-video":
		ok = decimalID.MatchString(id)
	case "playlist":
		ok = applePlaylistID.MatchString(id)
	}
	return
}

func dailymotionContentID(u *url.URL) string {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if parts[0] == "player" {
		return u.Query().Get("video")
	}
	id, _, _ := strings.Cut(parts[len(parts)-1], "_")
	return id
}

// Some providers publish an opaque player ID that cannot be derived from the
// page URL. Where the public ID is known, require the exact same content.
func providerPlayerMatches(name string, source, frame *url.URL) bool {
	s := strings.Split(strings.Trim(source.Path, "/"), "/")
	f := strings.Split(strings.Trim(frame.Path, "/"), "/")
	switch name {
	case "dbdiagram", "slides", "quizlet", "openstreetmap", "yandex-maps", "padlet":
		return appFrameMatches(name, source, frame)
	case "google-docs", "google-calendar", "google-maps":
		_, _, _, ok := googlePlayer(name, source, frame)
		return ok
	case "youtube":
		id := s[len(s)-1]
		if source.Path == "/watch" {
			id = source.Query().Get("v")
		}
		return f[len(f)-1] == id
	case "vimeo", "loom", "wistia", "ted", "prezi", "scribd":
		id := s[len(s)-1]
		if name == "prezi" {
			id = s[0]
		} else if name == "scribd" {
			return f[1] == s[1]
		}
		return f[len(f)-1] == id
	case "spotify":
		if strings.HasPrefix(s[0], "intl-") {
			s = s[1:]
		}
		return len(f) == 3 && f[1] == s[0] && f[2] == s[1]
	case "dailymotion":
		id := dailymotionContentID(source)
		if frame.Hostname() == "geo.dailymotion.com" {
			return frame.Query().Get("video") == id
		}
		return f[len(f)-1] == id
	case "codepen":
		return f[0] == s[0] && f[len(f)-1] == s[2]
	case "jsfiddle":
		return strings.HasPrefix(frame.Path, "/"+strings.Join(s, "/")+"/embedded/")
	case "deezer":
		return f[len(f)-2] == s[len(s)-2] && f[len(f)-1] == s[len(s)-1]
	case "art19":
		return strings.TrimSuffix(frame.Path, "/embed") == "/"+strings.Join(s, "/")
	case "geogebra":
		id := s[1]
		if s[0] == "student" {
			id = strings.TrimPrefix(id, "m")
		}
		return f[len(f)-1] == id
	case "documentcloud":
		sourceID, _, _ := strings.Cut(s[1], "-")
		frameID, _, _ := strings.Cut(f[1], "-")
		return sourceID == frameID
	case "applemusic":
		sourceKind, sourceID, sourceOK := appleMusicContent(source)
		frameKind, frameID, frameOK := appleMusicContent(frame)
		return sourceOK && frameOK && sourceKind == frameKind && sourceID == frameID
	}
	return true
}

// ControlledPlayer binds a public source to its own provider's exact HTTPS
// player endpoint. Its result is safe to hand to that provider's UI adapter.
// Query values are rebuilt where they affect geometry or embedding authority.
func ControlledPlayer(source, candidate, appURL string) (frame string, height int, ratio float64, ok bool) {
	u, err := mediafetch.URL(source)
	if err != nil {
		return
	}
	preserveMapFragment(u, source)
	p := publicProviderFor(u)
	if p == nil {
		return
	}
	if strings.HasPrefix(candidate, "//") {
		candidate = "https:" + candidate
	}
	f, err := mediafetch.URL(candidate)
	app, _ := url.Parse(appURL)
	if err != nil || f.Scheme != "https" || f.Port() != "" && f.Port() != "443" || f.RawPath != "" || strings.Contains(f.Path, "\\") || !publicProviderQuery(f, 0) || app != nil && strings.EqualFold(f.Hostname(), app.Hostname()) || !p.framePattern.MatchString(strings.ToLower(f.Hostname())+f.Path) || !providerPlayerMatches(p.name, u, f) {
		return
	}
	f.Fragment = ""
	input := f.Query()
	q := url.Values{}
	height, ratio = p.height, p.ratio
	switch p.name {
	case "dbdiagram", "slides", "quizlet", "openstreetmap", "yandex-maps", "padlet":
		target, valid := appFrameRoute(p.name, u)
		if !valid {
			return "", 0, 0, false
		}
		return target.String(), height, ratio, true
	case "google-docs", "google-calendar", "google-maps":
		return googlePlayer(p.name, u, f)
	case "spotify":
		if strings.Contains(f.Path, "/track/") || strings.Contains(f.Path, "/episode/") {
			height = 152
		}
	case "soundcloud":
		if !publicSoundCloudTarget(input.Get("url")) {
			return "", 0, 0, false
		}
		target, _ := mediafetch.URL(input.Get("url"))
		if strings.Contains(u.Path, "/sets/") {
			height = 450
		}
		q = url.Values{"url": {target.String()}, "visual": {"false"}, "auto_play": {"false"}, "show_comments": {"false"}}
	case "mixcloud":
		q = url.Values{"feed": {u.Path}, "hide_cover": {"1"}}
	case "bandcamp":
		ids := map[string]string{}
		for _, part := range strings.Split(f.Path, "/") {
			k, v, _ := strings.Cut(part, "=")
			if (k == "album" || k == "track") && decimalID.MatchString(v) {
				ids[k] = v
			}
		}
		if len(ids) == 0 {
			return "", 0, 0, false
		}
		f.Path = "/EmbeddedPlayer/"
		for _, k := range []string{"album", "track"} {
			if ids[k] != "" {
				f.Path += k + "=" + ids[k] + "/"
			}
		}
		f.Path += "size=large/bgcol=ffffff/linkcol=333333/artwork=small/tracklist=false/transparent=true/"
		q = url.Values{}
	case "dailymotion":
		if f.Hostname() == "geo.dailymotion.com" {
			if !providerID.MatchString(input.Get("video")) {
				return "", 0, 0, false
			}
			q.Set("video", input.Get("video"))
		}
	case "twitch":
		q = url.Values{}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if u.Hostname() == "clips.twitch.tv" || len(parts) == 3 && parts[1] == "clip" {
			f.Host, f.Path = "clips.twitch.tv", "/embed"
			q.Set("clip", parts[len(parts)-1])
		} else if len(parts) >= 2 {
			f.Host, f.Path = "player.twitch.tv", "/"
			q.Set("video", parts[len(parts)-1])
		} else {
			f.Host, f.Path = "player.twitch.tv", "/"
			q.Set("channel", parts[0])
		}
		q.Set("autoplay", "false")
		if app != nil && app.Hostname() != "" {
			q.Set("parent", app.Hostname())
		}
	case "applemusic":
		if strings.Contains(u.Path, "/song/") || decimalID.MatchString(u.Query().Get("i")) {
			height = 175
		}
		if strings.Contains(u.Path, "/music-video/") {
			height, ratio = 270, 16.0/9
		}
		if decimalID.MatchString(u.Query().Get("i")) {
			q.Set("i", u.Query().Get("i"))
		}
	case "codepen":
		q.Set("height", "400")
	case "livecodes":
		// The editor's source query defines the public example. Never adopt
		// another example or imported URL from arbitrary oEmbed markup.
		q = u.Query()
		q.Del("height")
		q.Del("width")
		q.Set("embed", "true")
		q.Set("loading", "click")
	case "art19":
		q = url.Values{"theme": {"light-gray-blue"}}
	case "libsyn":
		parts := strings.Split(strings.Trim(f.Path, "/"), "/")
		f.Host = "html5-player.libsyn.com"
		f.Path = "/embed/episode/id/" + parts[3] + "/theme/custom/height/96/thumbnail/yes/direction/backward/"
		q = url.Values{}
	case "simplecast":
		q = url.Values{}
	case "geogebra", "documentcloud", "prezi", "scribd":
		q = url.Values{}
	}
	if app != nil && strings.EqualFold(f.Hostname(), app.Hostname()) {
		return "", 0, 0, false
	}
	f.RawQuery = q.Encode()
	return f.String(), height, ratio, true
}

func providerMarkup(source, markup string) string {
	z := parse.NewTokenizer(strings.NewReader(markup))
	for {
		switch z.Next() {
		case parse.ErrorToken:
			return ""
		case parse.StartTagToken, parse.SelfClosingTagToken:
			token := z.Token()
			if token.Data != "iframe" {
				continue
			}
			for _, attr := range token.Attr {
				if attr.Key == "src" {
					if frame, _, _, ok := ControlledPlayer(source, attr.Val, ""); ok {
						return fmt.Sprintf(`<iframe src="%s"></iframe>`, html.EscapeString(frame))
					}
				}
			}
		}
	}
}

func providerEndpoint(raw string) string {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return ""
	}
	p := publicProviderFor(u)
	if p == nil {
		return ""
	}
	endpoint := ""
	switch p.name {
	case "soundcloud":
		u.Host = "soundcloud.com"
		endpoint = "https://soundcloud.com/oembed"
	case "mixcloud":
		endpoint = "https://www.mixcloud.com/oembed/"
	case "wistia":
		endpoint = "https://fast.wistia.com/oembed"
	case "ted":
		u.Path = strings.Replace(u.Path, "/dubbing/", "/talks/", 1)
		endpoint = "https://www.ted.com/services/v1/oembed.json"
	case "speakerdeck":
		endpoint = "https://speakerdeck.com/oembed.json"
	case "slideshare":
		endpoint = "https://www.slideshare.net/api/oembed/2"
	case "codepen":
		u.Path = strings.Replace(strings.Replace(u.Path, "/details/", "/pen/", 1), "/full/", "/pen/", 1)
		endpoint = "https://codepen.io/api/oembed"
	case "loom":
		endpoint = "https://www.loom.com/v1/oembed"
	case "documentcloud":
		endpoint = "https://api.www.documentcloud.org/api/oembed.json"
	case "scribd":
		endpoint = "https://www.scribd.com/services/oembed/"
	}
	if endpoint == "" {
		return ""
	}
	u.Scheme = "https"
	return endpoint + "?format=json&url=" + url.QueryEscape(u.String())
}
