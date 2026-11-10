package linkmedia

// Reviewed app conventions adapted from Iframely plugins/domains at
// 962bf9287f2ab5880ff96d4ac796ebb62217be57 (MIT).
// Copyright (c) 2013-2022 Itteco Software Corp.
// See iframely-providers.LICENSE.txt. Current primary evidence and exclusions
// are recorded in the ignored .data/embed-review/b4-provider-handoff.json.

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/artyom/oembed"
	parse "golang.org/x/net/html"
)

var oembedAppHexID = regexp.MustCompile(`^[a-f0-9]{24}$`)
var issuuSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,200}$`)
var cartoMapPath = regexp.MustCompile(`^/(?:u/[A-Za-z0-9_-]{1,100}/)?viz/[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}/(?:public_map|embed_map)$`)
var cartoTenant = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.carto\.com$`)

// Recognize unsupported members too, so a registry/discovery response cannot
// turn a private, removed or unreviewed app route into generic active markup.
func oembedAppFamily(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "player.cnevids.com", "player-backend.cnevids.com":
		return "cnevids"
	case "issuu.com", "www.issuu.com", "e.issuu.com":
		return "issuu"
	case "widgetic.com", "www.widgetic.com", "app.widgetic.com":
		return "widgetic"
	case "kickstarter.com", "www.kickstarter.com":
		return "kickstarter"
	}
	if strings.HasSuffix(host, ".carto.com") || strings.HasSuffix(host, ".cartodb.com") {
		return "carto"
	}
	return ""
}

func oembedAppRoute(name string, u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || u.RawPath != "" || u.Fragment != "" || u.Port() != "" && u.Port() != "443" || strings.Contains(u.Path, "\\") {
		return false
	}
	p := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	switch name {
	case "cnevids":
		return (u.Hostname() == "player.cnevids.com" || u.Hostname() == "player-backend.cnevids.com") && appFrameQuery(u, "autoplay") &&
			len(p) == 3 && (p[0] == "embed" && oembedAppHexID.MatchString(p[1]) && oembedAppHexID.MatchString(p[2]) || p[0] == "iframe" && p[1] == "video" && oembedAppHexID.MatchString(p[2]))
	case "issuu":
		return u.Hostname() == "issuu.com" && u.RawQuery == "" && !u.ForceQuery && len(p) == 3 && p[1] == "docs" && issuuSegment.MatchString(p[0]) && issuuSegment.MatchString(p[2]) && p[0] != "." && p[0] != ".." && p[2] != "." && p[2] != ".."
	case "carto":
		return cartoTenant.MatchString(u.Hostname()) && cartoMapPath.MatchString(u.Path) && u.RawQuery == "" && !u.ForceQuery
	case "widgetic":
		return u.Hostname() == "widgetic.com" && len(p) == 2 && p[0] == "embed" && oembedAppHexID.MatchString(p[1]) && appFrameQuery(u, "bp") && (!u.Query().Has("bp") || u.Query().Get("bp") == "top-right")
	}
	return false
}

func cneVideoID(u *url.URL) string {
	p := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(p) == 3 && p[0] == "embed" {
		return p[1]
	}
	if len(p) == 3 && p[0] == "iframe" && p[1] == "video" {
		return p[2]
	}
	return ""
}

func oembedAppPlayerMatches(name string, source, frame *url.URL) bool {
	if !oembedAppRoute(name, source) || frame.Fragment != "" {
		return false
	}
	switch name {
	case "cnevids":
		return frame.Hostname() == "player-backend.cnevids.com" && frame.Path == "/iframe/video/"+cneVideoID(source) && appFrameQuery(frame, "autoplay")
	case "issuu":
		p := strings.Split(strings.TrimPrefix(source.Path, "/"), "/")
		q := frame.Query()
		return frame.Hostname() == "e.issuu.com" && frame.Path == "/embed.html" && appFrameQuery(frame, "u|d") && len(q["u"]) == 1 && len(q["d"]) == 1 && q.Get("u") == p[0] && q.Get("d") == p[2]
	case "carto":
		expected := source.Path
		if strings.HasSuffix(expected, "/public_map") {
			expected = strings.TrimSuffix(expected, "/public_map") + "/embed_map"
		}
		return oembedAppRoute(name, frame) && frame.Hostname() == source.Hostname() && frame.Path == expected
	case "widgetic":
		return oembedAppRoute(name, frame) && source.Path == frame.Path
	}
	return false
}

func oembedAppEndpoint(u *url.URL) bool {
	return u != nil && (u.Hostname() == "player.cnevids.com" && u.Path == "/services/oembed" || u.Hostname() == "issuu.com" && u.Path == "/oembed")
}

// Wrappers, inline sizes, events and permissions are never retained. Multiple
// frames, srcdoc and script-dependent output cannot establish a player.
func oembedAppMarkup(source, markup string) string {
	if len(markup) > maxWidgetBytes {
		return ""
	}
	z := parse.NewTokenizer(strings.NewReader(markup))
	z.SetMaxBuf(maxWidgetBytes)
	frame, frames := "", 0
	for {
		switch z.Next() {
		case parse.ErrorToken:
			if z.Err() == io.EOF && frames == 1 && frame != "" {
				return `<iframe src="` + html.EscapeString(frame) + `"></iframe>`
			}
			return ""
		case parse.StartTagToken, parse.SelfClosingTagToken:
			t := z.Token()
			if t.Data == "script" || t.Data == "object" || t.Data == "embed" {
				return ""
			}
			if t.Data != "iframe" {
				continue
			}
			frames++
			srcs := 0
			for _, a := range t.Attr {
				if a.Key == "srcdoc" {
					return ""
				}
				if a.Key == "src" {
					srcs++
					var ok bool
					frame, _, _, ok = ControlledPlayer(source, a.Val, "")
					if !ok {
						return ""
					}
				}
			}
			if frames != 1 || srcs != 1 {
				return ""
			}
		}
	}
}

// Fetch only anonymous, bounded public HTML. A successful login redirect,
// different tenant or same-site content substitution is still a restriction.
func (r *Resolver) oembedAppPage(ctx context.Context, target *url.URL, frame bool) ([]byte, string) {
	res, err := r.fetch(ctx, target.String(), "text/html, application/xhtml+xml;q=0.9")
	if err != nil {
		return nil, failureReason(err)
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return nil, "restricted"
	}
	if res.StatusCode != http.StatusOK {
		return nil, "fetch_failed"
	}
	if res.Request == nil || res.Request.URL == nil || res.Request.URL.String() != target.String() || frame && !publicFrameHeaders(res.Header) {
		return nil, "restricted"
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxPageBytes+1))
	if err != nil || len(data) > maxPageBytes || !appHTMLType(res.Header) || !isHTML(data) {
		return nil, "fetch_failed"
	}
	return data, ""
}

func (r *Resolver) resolveOEmbedApp(ctx context.Context, raw, name string) (Result, error) {
	fallback := func(title, reason string) (Result, error) {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return infoCard(raw, title, "", reason, restrictedCacheTTL), nil
	}
	u, err := url.Parse(raw)
	if err != nil || !oembedAppRoute(name, u) {
		return fallback("", "unsupported_type")
	}
	// Source query options have no authority over the renderer or its size.
	u.RawQuery = ""
	if name == "widgetic" {
		u.RawQuery = "bp=top-right"
	}
	data, reason := r.oembedAppPage(ctx, u, name == "widgetic" || name == "carto" && strings.HasSuffix(u.Path, "/embed_map"))
	if reason != "" {
		return fallback("", reason)
	}
	m := readAppFrameMetadata(data)
	var meta *oembed.Metadata
	ttl := 24 * time.Hour
	if name == "cnevids" || name == "issuu" {
		if name == "issuu" && m.canonical != u.String() {
			return fallback(m.title, "restricted")
		}
		meta, ttl, err = r.fetchOEmbed(ctx, providerEndpoint(u.String()))
		if err != nil {
			return fallback(m.title, failureReason(err))
		}
		if meta.Type != oembed.TypeVideo && meta.Type != oembed.TypeRich || cleanText(meta.Title, 300) == "" {
			return fallback(m.title, "unsupported_type")
		}
		meta.HTML = oembedAppMarkup(raw, meta.HTML)
		if meta.HTML == "" {
			return fallback(meta.Title, "unsupported_type")
		}
		// Keep the generic fetcher and CDN thumbnail ingestion. Only a rebuilt
		// same-content player reaches the existing widget delivery policy.
		z := parse.NewTokenizer(strings.NewReader(meta.HTML))
		z.Next()
		for _, a := range z.Token().Attr {
			if a.Key == "src" {
				target, _ := url.Parse(a.Val)
				if _, reason = r.oembedAppPage(ctx, target, true); reason != "" {
					return fallback(meta.Title, reason)
				}
			}
		}
	} else {
		target := *u
		if name == "carto" {
			target.Path = strings.TrimSuffix(target.Path, "/public_map")
			if !strings.HasSuffix(target.Path, "/embed_map") {
				target.Path += "/embed_map"
				if !oembedAppLink(data, target.String()) {
					return fallback(m.title, "unsupported_type")
				}
				data, reason = r.oembedAppPage(ctx, &target, true)
				if reason != "" {
					return fallback(m.title, reason)
				}
				m = readAppFrameMetadata(data)
			}
			if m.title == "" || m.canonical != target.String() {
				return fallback(m.title, "restricted")
			}
		} else if !strings.Contains(string(data), "'/api/v2/compositions/"+strings.TrimPrefix(u.Path, "/embed/")+"/embed/data'") {
			// Inspect the public embed shell as data; do not call composition
			// account APIs, execute its controller or copy its SDK into IRCord.
			return fallback(m.title, "unsupported_type")
		}
		frame, _, _, ok := ControlledPlayer(raw, target.String(), "")
		if !ok {
			return fallback(m.title, "unsupported_type")
		}
		site := "Widgetic"
		if name == "carto" {
			site = "CARTO"
		}
		meta = &oembed.Metadata{Type: oembed.TypeRich, Title: m.title, Provider: site, Thumbnail: metadataURL(m.image, &target), HTML: `<iframe src="` + html.EscapeString(frame) + `"></iframe>`}
	}
	return r.oembedResult(ctx, raw, meta, ttl)
}

func oembedAppLink(data []byte, target string) bool {
	z := parse.NewTokenizer(strings.NewReader(string(data)))
	z.SetMaxBuf(maxPageBytes)
	for {
		switch z.Next() {
		case parse.ErrorToken:
			return false
		case parse.StartTagToken:
			t := z.Token()
			if t.Data == "a" {
				for _, a := range t.Attr {
					if a.Key == "href" && a.Val == target {
						return true
					}
				}
			}
		}
	}
}
