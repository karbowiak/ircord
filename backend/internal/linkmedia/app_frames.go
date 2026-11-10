package linkmedia

// Controlled app/map conventions adapted from Iframely plugins/domains at
// 962bf9287f2ab5880ff96d4ac796ebb62217be57 (MIT, Itteco 2013-2022).
// See iframely-providers.LICENSE.txt. Scripts, private tokens, obsolete APIs and
// provider-driven resizing are excluded. Current official references are kept
// in the ignored frames-provider-handoff.json review packet.

import (
	"context"
	"encoding/xml"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/artyom/oembed"
	parse "golang.org/x/net/html"
	"ircord/backend/internal/mediafetch"
)

var dbdiagramSlug = regexp.MustCompile(`^(?:[A-Za-z0-9-]+-)?([A-Fa-f0-9]{24})$`)
var quizletLocale = regexp.MustCompile(`^[a-z]{2}$`)
var quizletSlugID = regexp.MustCompile(`^[A-Za-z0-9_-]+-([0-9]{1,30})$`)
var padletOpaqueID = regexp.MustCompile(`^[a-z0-9]{12,32}$`)
var yandexShortID = regexp.MustCompile(`^[A-Za-z0-9_~.-]{1,100}$`)
var yandexPlaceURI = regexp.MustCompile(`^ymapsbm1://org\?oid=[0-9]{1,30}$`)
var slidesPublicVisibility = regexp.MustCompile(`"visibility"\s*:\s*"all"`)
var osmNodePath = regexp.MustCompile(`^/node/[0-9]{1,30}$`)

func preserveMapFragment(u *url.URL, raw string) {
	if u != nil && (u.Hostname() == "openstreetmap.org" || u.Hostname() == "www.openstreetmap.org") {
		if original, err := url.Parse(raw); err == nil {
			u.Fragment = original.Fragment
		}
	}
}

func appFrameQuery(u *url.URL, allowed string) bool {
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(u.RawQuery) > 4096 || len(q) > 24 || !publicProviderQuery(u, 0) {
		return false
	}
	for key, values := range q {
		if !strings.Contains("|"+allowed+"|", "|"+key+"|") || len(values) != 1 {
			return false
		}
	}
	return true
}

func appFrameRoute(name string, u *url.URL) (*url.URL, bool) {
	if u == nil || u.Scheme != "https" || u.User != nil || u.RawPath != "" || u.Port() != "" && u.Port() != "443" || strings.Contains(u.Path, "\\") {
		return nil, false
	}
	p := strings.Split(strings.Trim(u.Path, "/"), "/")
	target := &url.URL{Scheme: "https", Host: u.Hostname()}
	switch name {
	case "dbdiagram":
		if u.Hostname() != "dbdiagram.io" || !appFrameQuery(u, "") || len(p) != 2 || p[0] != "d" && p[0] != "embed" {
			return nil, false
		}
		id := dbdiagramSlug.FindStringSubmatch(p[1])
		if id == nil {
			return nil, false
		}
		target.Path = "/embed/" + id[1]
	case "slides":
		if u.Hostname() != "slides.com" || !appFrameQuery(u, "") || len(p) != 2 && !(len(p) == 3 && p[2] == "embed") || !providerID.MatchString(p[0]) || !providerID.MatchString(p[1]) {
			return nil, false
		}
		target.Path = "/" + p[0] + "/" + p[1] + "/embed"
	case "quizlet":
		if u.Hostname() != "quizlet.com" || !appFrameQuery(u, "") {
			return nil, false
		}
		if len(p) >= 3 && quizletLocale.MatchString(p[0]) {
			p = p[1:]
		}
		id, mode := "", "flashcards"
		if len(p) == 2 && decimalID.MatchString(p[0]) {
			id = p[0]
			if p[1] == "scatter" || p[1] == "gravity" {
				return nil, false // retired activities are not silently changed
			}
			if quizletMode(p[1]) {
				mode = p[1]
			}
		} else if len(p) == 2 && quizletMode(p[0]) {
			match := quizletSlugID.FindStringSubmatch(p[1])
			if match != nil {
				id, mode = match[1], p[0]
			}
		} else if len(p) == 3 && decimalID.MatchString(p[0]) && quizletMode(p[1]) && p[2] == "embed" {
			id, mode = p[0], p[1]
		}
		if id == "" {
			return nil, false
		}
		target.Path = "/" + id + "/" + mode + "/embed"
	case "openstreetmap":
		return openStreetMapFrame(u)
	case "yandex-maps":
		if u.Hostname() != "yandex.ru" && u.Hostname() != "yandex.com" || !appFrameQuery(u, "ll|z|mode|poi[point]|poi[uri]|sctx|sll|sspn|text|utm_source|utm_medium|from|origin|lang|l") {
			return nil, false
		}
		if len(p) == 3 && p[0] == "maps" && p[1] == "-" && yandexShortID.MatchString(p[2]) {
			target.Path = "/map-widget/v1/-/" + p[2]
		} else if len(p) == 4 && p[0] == "map-widget" && p[1] == "v1" && p[2] == "-" && yandexShortID.MatchString(p[3]) {
			target.Path = u.Path
		} else if p[0] == "maps" || strings.TrimSuffix(u.Path, "/") == "/map-widget/v1" {
			ll, ok := coordinatePair(u.Query().Get("ll"), true)
			zoomText := u.Query().Get("z")
			if zoomText == "" {
				zoomText = "7"
			}
			zoom, err := strconv.Atoi(zoomText)
			if !ok || err != nil || zoom < 0 || zoom > 21 {
				return nil, false
			}
			target.Path = "/map-widget/v1/"
			target.RawQuery = url.Values{"ll": {ll}, "z": {strconv.Itoa(zoom)}}.Encode()
		} else {
			return nil, false
		}
	case "padlet":
		if u.Hostname() != "padlet.com" || !appFrameQuery(u, "") || len(p) != 2 || !providerID.MatchString(p[0]) || p[0] == "auth" || p[0] == "login" || p[0] == "dashboard" {
			return nil, false
		}
		id := p[1]
		if i := strings.LastIndexByte(id, '-'); i >= 0 {
			id = id[i+1:]
		}
		if !padletOpaqueID.MatchString(id) {
			return nil, false // old human aliases cannot bind an opaque frame ID
		}
		target.Path = "/embed/" + id
	default:
		return nil, false
	}
	return target, true
}

func quizletMode(mode string) bool {
	return strings.Contains("|flashcards|match|learn|spell|test|", "|"+mode+"|")
}

func coordinate(value string, limit float64) (float64, bool) {
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= -limit && n <= limit
}

func mapNumber(n float64) string { return strconv.FormatFloat(n, 'f', 7, 64) }

func coordinatePair(raw string, lonFirst bool) (string, bool) {
	p := strings.Split(raw, ",")
	if len(p) != 2 {
		return "", false
	}
	aLimit, bLimit := 90.0, 180.0
	if lonFirst {
		aLimit, bLimit = bLimit, aLimit
	}
	a, ok := coordinate(p[0], aLimit)
	b, valid := coordinate(p[1], bLimit)
	return mapNumber(a) + "," + mapNumber(b), ok && valid
}

func openStreetMapFrame(u *url.URL) (*url.URL, bool) {
	if u.Hostname() != "openstreetmap.org" && u.Hostname() != "www.openstreetmap.org" ||
		!appFrameQuery(u, "lat|lon|zoom|layers|layer|mlat|mlon|minlat|minlon|maxlat|maxlon|bbox|marker") {
		return nil, false
	}
	path := strings.TrimSuffix(u.Path, "/")
	if path != "" && path != "/export/embed.html" && !osmNodePath.MatchString(path) {
		return nil, false
	}
	q := u.Query()
	if !q.Has("bbox") && (q.Has("minlat") || q.Has("minlon") || q.Has("maxlat") || q.Has("maxlon")) {
		q.Set("bbox", strings.Join([]string{q.Get("minlon"), q.Get("minlat"), q.Get("maxlon"), q.Get("maxlat")}, ","))
	}
	layer := q.Get("layer")
	if layer != "" && layer != "mapnik" || q.Get("layers") != "" && strings.Trim(q.Get("layers"), "MOCND") != "" {
		return nil, false // unreviewed third-party tile layers are not promoted
	}
	bbox := ""
	if raw := q.Get("bbox"); raw != "" {
		p := strings.Split(raw, ",")
		if len(p) != 4 {
			return nil, false
		}
		values := make([]float64, 4)
		for i, value := range p {
			limit := 180.0
			if i%2 == 1 {
				limit = 85.05112878
			}
			var ok bool
			if values[i], ok = coordinate(value, limit); !ok {
				return nil, false
			}
		}
		if values[0] >= values[2] || values[1] >= values[3] {
			return nil, false
		}
		parts := make([]string, 4)
		for i, value := range values {
			parts[i] = mapNumber(value)
		}
		bbox = strings.Join(parts, ",")
	} else {
		lat, lon, zoom := q.Get("lat"), q.Get("lon"), q.Get("zoom")
		if u.Fragment != "" {
			fragment, err := url.ParseQuery(u.Fragment)
			if err != nil || len(fragment) > 2 || len(fragment["map"]) != 1 || len(fragment["layers"]) > 1 {
				return nil, false
			}
			for key := range fragment {
				if key != "map" && key != "layers" {
					return nil, false
				}
			}
			if strings.Trim(fragment.Get("layers"), "MOCND") != "" {
				return nil, false
			}
			p := strings.Split(fragment.Get("map"), "/")
			if len(p) != 3 {
				return nil, false
			}
			zoom, lat, lon = p[0], p[1], p[2]
		}
		if lat == "" && lon == "" {
			lat, lon = q.Get("mlat"), q.Get("mlon")
		}
		if zoom == "" {
			zoom = "10"
		}
		a, ok := coordinate(lat, 85.05112878)
		b, valid := coordinate(lon, 180)
		z, err := strconv.Atoi(zoom)
		if !ok || !valid || err != nil || z < 0 || z > 18 {
			return nil, false
		}
		// A fixed 640x480 reference viewport preserves the map's content extent;
		// iframe/card geometry remains the independently selected 4:3 box.
		scale := 256 * math.Exp2(float64(z))
		x := (b + 180) / 360
		y := (1 - math.Asinh(math.Tan(a*math.Pi/180))/math.Pi) / 2
		west, east := math.Max(-180, (x-320/scale)*360-180), math.Min(180, (x+320/scale)*360-180)
		south := math.Atan(math.Sinh(math.Pi*(1-2*math.Min(1, y+240/scale)))) * 180 / math.Pi
		north := math.Atan(math.Sinh(math.Pi*(1-2*math.Max(0, y-240/scale)))) * 180 / math.Pi
		bbox = strings.Join([]string{mapNumber(west), mapNumber(south), mapNumber(east), mapNumber(north)}, ",")
	}
	out := url.Values{"bbox": {bbox}, "layer": {"mapnik"}}
	marker := q.Get("marker")
	if marker == "" && (q.Has("mlat") || q.Has("mlon")) {
		marker = q.Get("mlat") + "," + q.Get("mlon")
	}
	if marker != "" {
		pair, ok := coordinatePair(marker, false)
		if !ok {
			return nil, false
		}
		out.Set("marker", pair)
	}
	return &url.URL{Scheme: "https", Host: "www.openstreetmap.org", Path: "/export/embed.html", RawQuery: out.Encode()}, true
}

func appFrameMatches(name string, source, frame *url.URL) bool {
	expected, ok := appFrameRoute(name, source)
	if !ok {
		return false
	}
	if name == "yandex-maps" && strings.Contains(expected.Path, "/-/") && (frame.Hostname() == "api-maps.yandex.com" || frame.Hostname() == "api-maps.yandex.ru") {
		return frame.Path == strings.Replace(expected.Path, "/map-widget/v1/", "/frame/v1/", 1) && frame.RawQuery == ""
	}
	actual, valid := appFrameRoute(name, frame)
	return valid && expected.String() == actual.String()
}

func appFrameSource(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "dbdiagram.io", "slides.com", "quizlet.com", "openstreetmap.org", "www.openstreetmap.org", "padlet.com":
		return true
	case "yandex.ru", "yandex.com":
		return strings.HasPrefix(u.Path, "/maps/") || strings.HasPrefix(u.Path, "/map-widget/")
	}
	return false
}

type appFrameMetadata struct {
	pageMetadata
	canonical, padletID string
}

func readAppFrameMetadata(data []byte) appFrameMetadata {
	m := appFrameMetadata{pageMetadata: readPageMetadata(data)}
	z := parse.NewTokenizer(strings.NewReader(string(data)))
	z.SetMaxBuf(maxPageBytes)
	for {
		switch z.Next() {
		case parse.ErrorToken:
			return m
		case parse.StartTagToken, parse.SelfClosingTagToken:
			t := z.Token()
			attrs := map[string]string{}
			for _, a := range t.Attr {
				attrs[a.Key] = a.Val
			}
			if t.Data == "link" && attrs["rel"] == "canonical" {
				m.canonical = attrs["href"]
			}
			if t.Data == "meta" && attrs["property"] == "og:url" && m.canonical == "" {
				m.canonical = attrs["content"]
			}
			if t.Data == "link" && attrs["rel"] == "alternate" {
				feed, err := url.Parse(attrs["href"])
				if err != nil || feed.Scheme != "https" || feed.Host != "padlet.com" || feed.User != nil || feed.RawPath != "" {
					continue
				}
				p := strings.Split(strings.Trim(feed.Path, "/"), "/")
				if len(p) == 4 && p[0] == "padlets" && p[2] == "exports" && p[3] == "feed.xml" && padletOpaqueID.MatchString(p[1]) {
					// Only the path ID is retained. Feed tokens are never fetched,
					// logged, copied, or used to establish public eligibility.
					m.padletID = p[1]
				}
			}
		}
	}
}

func (r *Resolver) resolveAppFramePage(ctx context.Context, raw string) (Result, error) {
	fallback := func(title, reason string) (Result, error) {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return infoCard(raw, title, "", reason, 5*time.Minute), nil
	}
	u, err := mediafetch.URL(raw)
	if err != nil {
		return Result{}, err
	}
	preserveMapFragment(u, raw)
	p := publicProviderFor(u)
	if p == nil {
		return fallback("", "unsupported_type")
	}
	target, _ := appFrameRoute(p.name, u)
	fetchURL := target.String()
	if p.name == "slides" {
		fetchURL = strings.TrimSuffix(fetchURL, "/embed")
	} else if p.name == "quizlet" || p.name == "dbdiagram" || p.name == "padlet" {
		fetchURL = raw
	}
	page, err := r.fetch(ctx, fetchURL, "text/html, application/xhtml+xml;q=0.9")
	if err != nil {
		return fallback("", failureReason(err))
	}
	defer page.Body.Close()
	if page.StatusCode == 401 || page.StatusCode == 403 {
		return fallback("", "restricted")
	}
	if page.StatusCode != 200 || page.Request == nil || page.Request.URL == nil {
		return fallback("", "fetch_failed")
	}
	final := page.Request.URL
	if final.RawPath != "" || final.User != nil || !publicProviderQuery(final, 0) || !appFrameMatches(p.name, u, final) {
		return fallback("", "restricted")
	}
	if fetchURL == target.String() && !publicFrameHeaders(page.Header) {
		return fallback("", "restricted")
	}
	data, err := io.ReadAll(io.LimitReader(page.Body, maxPageBytes+1))
	if err != nil || len(data) > maxPageBytes || !appHTMLType(page.Header) || !isHTML(data) {
		return fallback("", "fetch_failed")
	}
	m := readAppFrameMetadata(data)
	if p.name == "slides" || p.name == "quizlet" || p.name == "dbdiagram" {
		canonical, err := url.Parse(m.canonical)
		if err != nil || m.title == "" || !appFrameMatches(p.name, u, canonical) {
			return fallback(m.title, "unsupported_type")
		}
		if p.name == "slides" && !slidesPublicVisibility.Match(data) {
			return fallback(m.title, "restricted")
		}
		if p.name == "dbdiagram" {
			// The current upstream pages return an identical generic SPA shell
			// for public/private/missing IDs. That is not public-content proof.
			return fallback(m.title, "unsupported_type")
		}
	}
	if p.name == "padlet" {
		id := strings.TrimPrefix(target.Path, "/embed/")
		if m.title == "" || m.padletID != id {
			return fallback(m.title, "restricted")
		}
		if public, reason := r.publicPadletFeed(ctx, id); !public {
			return fallback(m.title, reason)
		}
	}
	if p.name == "quizlet" || p.name == "slides" {
		// A public source page does not imply that a login-only activity or
		// a site-restricted frame is usable anonymously.
		if public, reason := r.publicAppFrame(ctx, target, p.name); !public {
			return fallback(m.title, reason)
		}
	}
	frame, _, _, ok := ControlledPlayer(raw, target.String(), "")
	if !ok {
		return fallback(m.title, "unsupported_type")
	}
	return r.oembedResult(ctx, raw, &oembed.Metadata{Type: oembed.TypeRich, Provider: m.site, Title: m.title, HTML: `<iframe src="` + html.EscapeString(frame) + `"></iframe>`}, 24*time.Hour)
}

func (r *Resolver) publicAppFrame(ctx context.Context, target *url.URL, name string) (bool, string) {
	res, err := r.fetch(ctx, target.String(), "text/html")
	if err != nil {
		return false, failureReason(err)
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return false, "restricted"
	}
	if res.StatusCode != 200 || res.Request == nil || res.Request.URL == nil || !appFrameMatches(name, target, res.Request.URL) {
		return false, "fetch_failed"
	}
	if !publicFrameHeaders(res.Header) {
		return false, "restricted"
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxPageBytes+1))
	return err == nil && len(data) <= maxPageBytes && appHTMLType(res.Header) && isHTML(data), "fetch_failed"
}

func appHTMLType(header http.Header) bool {
	value := strings.TrimSpace(strings.ToLower(strings.SplitN(header.Get("Content-Type"), ";", 2)[0]))
	return value == "text/html" || value == "application/xhtml+xml"
}

func publicFrameHeaders(header http.Header) bool {
	xfo := strings.TrimSpace(strings.ToLower(header.Get("X-Frame-Options")))
	if xfo != "" && xfo != "allowall" {
		return false
	}
	for _, policy := range header.Values("Content-Security-Policy") {
		for _, directive := range strings.Split(strings.ToLower(policy), ";") {
			fields := strings.Fields(directive)
			if len(fields) > 0 && fields[0] == "frame-ancestors" && (len(fields) != 2 || fields[1] != "*") {
				return false
			}
		}
	}
	return true
}

func (r *Resolver) publicPadletFeed(ctx context.Context, id string) (bool, string) {
	target := "https://padlet.com/padlets/" + id + "/exports/feed.xml"
	res, err := r.fetch(ctx, target, "application/rss+xml, application/xml, text/xml")
	if err != nil {
		return false, failureReason(err)
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return false, "restricted"
	}
	if res.StatusCode != 200 || res.Request == nil || res.Request.URL == nil || res.Request.URL.String() != target {
		return false, "fetch_failed"
	}
	contentType := strings.TrimSpace(strings.ToLower(strings.SplitN(res.Header.Get("Content-Type"), ";", 2)[0]))
	if contentType != "application/rss+xml" && contentType != "application/xml" && contentType != "text/xml" {
		return false, "fetch_failed"
	}
	decoder := xml.NewDecoder(io.LimitReader(res.Body, maxPageBytes))
	for {
		token, err := decoder.Token()
		if err != nil {
			return false, "fetch_failed"
		}
		if start, ok := token.(xml.StartElement); ok {
			return start.Name.Local == "rss", "fetch_failed"
		}
	}
}
