package linkmedia

// Metadata/player conventions adapted from Iframely plugins/domains, commit
// 962bf9287f2ab5880ff96d4ac796ebb62217be57 (MIT; see accompanying notice).

import (
	"bytes"
	"context"
	"errors"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/artyom/oembed"
	parse "golang.org/x/net/html"
	"ircord/backend/internal/mediafetch"
)

// Only standard head fields are retained. The bounded parser neither executes
// scripts nor follows metadata refresh, arbitrary embed URLs, or JSON content.
type pageMetadata struct {
	title, description, image, player, video, site string
	comic, comicTitle                              string
	frames                                         []string
}

func readPageMetadata(data []byte) pageMetadata {
	m := pageMetadata{}
	z := parse.NewTokenizer(bytes.NewReader(data))
	z.SetMaxBuf(maxPageBytes)
	inComic := false
	for {
		switch z.Next() {
		case parse.ErrorToken:
			return m
		case parse.StartTagToken, parse.SelfClosingTagToken:
			t := z.Token()
			attrs := map[string]string{}
			for _, a := range t.Attr {
				if a.Namespace == "" {
					attrs[a.Key] = a.Val
				}
			}
			if attrs["id"] == "comic" {
				inComic = true
			}
			if t.Data == "img" && inComic && m.comic == "" {
				m.comic, m.comicTitle = attrs["src"], attrs["title"]
			}
			if t.Data == "iframe" && len(m.frames) < 8 && len(attrs["src"]) < 4096 {
				m.frames = append(m.frames, attrs["src"])
			}
			if t.Data != "meta" {
				continue
			}
			key := strings.ToLower(attrs["property"])
			if key == "" {
				key = strings.ToLower(attrs["name"])
			}
			value := attrs["content"]
			switch key {
			case "og:title":
				m.title = cleanText(value, 300)
			case "og:description", "twitter:description":
				if m.description == "" {
					m.description = cleanText(value, 1000)
				}
			case "og:site_name":
				m.site = cleanText(value, 120)
			case "og:image", "og:image:secure_url", "twitter:image", "twitter:image:src":
				if m.image == "" {
					m.image = value
				}
			case "twitter:player":
				m.player = value
			case "og:video", "og:video:url", "og:video:secure_url":
				if m.video == "" {
					m.video = value
				}
			}
		case parse.EndTagToken:
			if t := z.Token(); t.Data == "div" && inComic {
				inComic = false
			}
		}
	}
}

func metadataURL(raw string, base *url.URL) string {
	u, err := url.Parse(raw)
	if err != nil || raw == "" {
		return ""
	}
	u = base.ResolveReference(u)
	if _, err = mediafetch.URL(u.String()); err != nil || fileCapabilityURL(u.String()) {
		return ""
	}
	return u.String()
}

// NativeImageSource identifies page routes with a fixed native image adapter
// before resolution. Unknown oEmbed photos retain the metadata-card geometry.
func NativeImageSource(raw string) bool {
	if NativeProviderImageSource(raw) {
		return true
	}
	u, err := mediafetch.URL(raw)
	if err != nil || u.RawPath != "" || strings.Contains(u.Path, "\\") || !publicProviderQuery(u, 0) {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch host {
	case "xkcd.com":
		return len(parts) == 1 && decimalID.MatchString(parts[0])
	case "giphy.com":
		return len(parts) == 2 && (parts[0] == "gifs" || parts[0] == "stickers") && providerID.MatchString(parts[1])
	case "flickr.com":
		return len(parts) >= 3 && parts[0] == "photos" && parts[1] != "" && decimalID.MatchString(parts[2])
	case "flic.kr":
		return len(parts) == 2 && parts[0] == "p" && providerID.MatchString(parts[1])
	}
	return false
}

// directProviderPlayer uses a provider's documented player convention only
// after a successful public page fetch. It never manufactures credentials or
// falls back through a login, age gate, or unsuccessful HTTP response.
func directProviderPlayer(raw string, m pageMetadata) string {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return ""
	}
	p := publicProviderFor(u)
	if p == nil {
		return ""
	}
	if oembedAppFamily(raw) != "" {
		// These apps require their reviewed public-source and frame checks.
		// Page metadata alone cannot establish controlled player authority.
		return ""
	}
	for _, candidate := range append([]string{m.player, m.video}, m.frames...) {
		// Historical Bandcamp Twitter metadata used HTTP. Only upgrade that
		// exact known origin before validating its numeric IDs and layout.
		if p.name == "bandcamp" {
			if player, e := url.Parse(candidate); e == nil && player.Host == "bandcamp.com" && player.Scheme == "http" {
				player.Scheme = "https"
				candidate = player.String()
			}
		}
		if frame, _, _, ok := ControlledPlayer(raw, candidate, ""); ok {
			return frame
		}
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	frame := ""
	switch p.name {
	case "loom":
		frame = "https://www.loom.com/embed/" + parts[1]
	case "wistia":
		frame = "https://fast.wistia.net/embed/iframe/" + parts[1]
	case "twitch":
		frame = "https://player.twitch.tv/"
	case "ted":
		frame = "https://embed.ted.com/talks/" + parts[1]
	case "codepen":
		frame = "https://codepen.io/" + parts[0] + "/embed/preview/" + parts[2]
	case "jsfiddle":
		frame = "https://jsfiddle.net/" + strings.Trim(u.Path, "/") + "/embedded/"
	case "livecodes":
		frame = "https://" + u.Hostname() + "/?" + u.RawQuery
	case "applemusic":
		if len(parts[0]) != 2 {
			return "" // Locale-less legacy iTunes URLs need a real redirect.
		}
		parts[len(parts)-1] = strings.TrimPrefix(parts[len(parts)-1], "id")
		frame = "https://embed.music.apple.com/" + strings.Join(parts, "/")
		if decimalID.MatchString(u.Query().Get("i")) {
			frame += "?i=" + u.Query().Get("i")
		}
	case "deezer":
		if len(parts) == 3 {
			parts = parts[1:]
		}
		frame = "https://widget.deezer.com/widget/auto/" + strings.Join(parts, "/")
	case "art19":
		frame = "https://art19.com/" + strings.Join(parts, "/") + "/embed"
	case "geogebra":
		id := parts[1]
		if parts[0] == "student" {
			id = strings.TrimPrefix(id, "m")
		}
		if !decimalID.MatchString(id) {
			frame = "https://www.geogebra.org/material/iframe/id/" + id
		}
	case "documentcloud":
		id, _, _ := strings.Cut(parts[1], "-")
		frame = "https://embed.documentcloud.org/documents/" + id
	case "prezi":
		frame = "https://prezi.com/embed/" + parts[0] + "/"
	case "scribd":
		frame = "https://www.scribd.com/embeds/" + parts[1] + "/content"
	}
	if safe, _, _, ok := ControlledPlayer(raw, frame, ""); ok {
		return safe
	}
	return ""
}

func (r *Resolver) pageProviderResult(ctx context.Context, raw string, m pageMetadata) (Result, bool, error) {
	u, _ := url.Parse(raw)
	var assetURL string
	var assetErr error
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host == "xkcd.com" && decimalID.MatchString(strings.Trim(u.Path, "/")) {
		assetURL = metadataURL(m.comic, u)
		image, _ := url.Parse(assetURL)
		if image == nil || image.Hostname() != "imgs.xkcd.com" || !strings.HasPrefix(image.Path, "/comics/") {
			assetURL = ""
		}
	} else if host == "giphy.com" && (strings.HasPrefix(u.Path, "/gifs/") || strings.HasPrefix(u.Path, "/stickers/")) && m.title != "Private GIF" {
		assetURL = metadataURL(m.image, u)
		image, _ := url.Parse(assetURL)
		if image == nil || !(image.Hostname() == "media.giphy.com" || strings.HasSuffix(image.Hostname(), ".giphy.com")) {
			assetURL = ""
		}
	}
	if assetURL != "" {
		asset, err := r.ingest(ctx, Asset{URL: assetURL, Kind: "image"}, "")
		if err == nil {
			return Result{URL: raw, Provider: "oembed", Title: m.title, Description: m.comicTitle, Assets: []Asset{asset}, CacheUntil: asset.ExpiresAt}, true, nil
		}
		if errors.Is(err, ErrBusy) {
			return Result{}, false, err
		}
		assetErr = err
	}
	if frame := directProviderPlayer(raw, m); frame != "" {
		meta := &oembed.Metadata{Type: oembed.TypeVideo, Title: m.title, Provider: m.site, HTML: `<iframe src="` + html.EscapeString(frame) + `"></iframe>`, Thumbnail: metadataURL(m.image, u)}
		value, err := r.oembedResult(ctx, raw, meta, 24*time.Hour)
		return value, err == nil, err
	}
	return Result{}, false, assetErr
}
