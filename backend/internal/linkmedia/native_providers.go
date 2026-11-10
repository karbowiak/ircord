package linkmedia

// Native public-media conventions reviewed against Iframely plugins/domains
// d.pr.js, imageshack.com.js, smugmug.com.js and giphy.com.js at
// 962bf9287f2ab5880ff96d4ac796ebb62217be57 (MIT).
// Copyright (c) 2013-2022 Itteco Software Corp.
// See iframely-providers.LICENSE.txt. No upstream scripts execute here.
// 500px is deliberately excluded: the reviewed public fixture returned no
// usable metadata, and its legacy web.500px.com fixture failed TLS validation.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/artyom/oembed"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"ircord/backend/internal/mediafetch"
)

type nativeProvider struct {
	name, kind, id, host string
}

var nativePhotoID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
var droplrHost = regexp.MustCompile(`^(?:[a-z0-9-]+\.)?d\.pr$`)
var smugmugHost = regexp.MustCompile(`^[a-z0-9-]+\.smugmug\.com$`)
var giphyMediaHost = regexp.MustCompile(`^media[0-4]?\.giphy\.com$`)

func nativeProviderURL(raw string) (*url.URL, bool) {
	u, err := mediafetch.URL(raw)
	return u, err == nil && u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") && u.RawPath == "" && !strings.Contains(u.Path, "\\") && !strings.Contains(u.Path, "//") && !fileCapabilityURL(raw)
}

func nativeProviderSource(raw string) (nativeProvider, bool) {
	u, ok := nativeProviderURL(raw)
	if !ok || !publicProviderQuery(u, 0) || len(u.RawQuery) > 2048 {
		return nativeProvider{}, false
	}
	for key := range u.Query() {
		key = strings.ToLower(key)
		if !strings.HasPrefix(key, "utm_") && key != "cid" && key != "ref" {
			return nativeProvider{}, false
		}
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	p := nativeProvider{host: host}
	switch {
	case droplrHost.MatchString(host):
		if len(parts) > 0 && parts[0] == "free" {
			parts = parts[1:]
		}
		if len(parts) != 2 || !nativePhotoID.MatchString(parts[1]) || (parts[0] != "i" && parts[0] != "v") {
			return nativeProvider{}, false
		}
		p.name, p.kind, p.id = "droplr", "image", parts[1]
		if parts[0] == "v" {
			p.kind = "video"
		}
	case host == "imageshack.com" || host == "www.imageshack.com":
		if len(parts) != 2 || parts[0] != "i" || !nativePhotoID.MatchString(parts[1]) || len(parts[1]) < 4 {
			return nativeProvider{}, false
		}
		p.name, p.kind, p.id = "imageshack", "image", parts[1]
	case smugmugHost.MatchString(host) && host != "photos.smugmug.com" && host != "api.smugmug.com" && host != "www.smugmug.com":
		last := parts[len(parts)-1]
		if len(parts) > 12 || !strings.HasPrefix(last, "i-") || !nativePhotoID.MatchString(strings.TrimPrefix(last, "i-")) {
			return nativeProvider{}, false
		}
		p.name, p.kind, p.id = "smugmug", "image", last
	case host == "giphy.com" || host == "www.giphy.com":
		if len(parts) != 2 || (parts[0] != "clips" && parts[0] != "videos") {
			return nativeProvider{}, false
		}
		id := parts[1][strings.LastIndex(parts[1], "-")+1:]
		if !nativePhotoID.MatchString(id) {
			return nativeProvider{}, false
		}
		p.name, p.kind, p.id = "giphy-clips", "video", id
	default:
		return nativeProvider{}, false
	}
	return p, true
}

// NativeProviderSource is a routing hint for only the reviewed public page
// families. The resolver still requires matching successful public metadata,
// same-content CDN URLs, observed bytes, and validated CDN ingest.
func NativeProviderSource(raw string) bool {
	_, ok := nativeProviderSource(raw)
	return ok
}

func NativeProviderImageSource(raw string) bool {
	return NativeProviderMediaSource(raw, "image")
}

// NativeProviderMediaSource lets the API retain only the media kind reserved
// by the original page's static adapter, after its usual CDN-delivery checks.
func NativeProviderMediaSource(raw, kind string) bool {
	p, ok := nativeProviderSource(raw)
	return ok && p.kind == kind
}

func sameNativeProvider(source nativeProvider, raw string) bool {
	p, ok := nativeProviderSource(raw)
	return ok && p.name == source.name && p.kind == source.kind && p.id == source.id && (p.name != "smugmug" || p.host == source.host)
}

type nativeProviderMetadata struct {
	title, description, canonical, image, video, videoType, endpoint string
	videoWidth, videoHeight                                          int
}

// Only bounded standard head metadata is read. Scripts, body markup and
// arbitrary embedded JSON cannot grant access to a media source.
func readNativeProviderMetadata(data []byte) nativeProviderMetadata {
	m := nativeProviderMetadata{}
	z := html.NewTokenizer(bytes.NewReader(data))
	z.SetMaxBuf(maxPageBytes)
	for {
		switch z.Next() {
		case html.ErrorToken:
			return m
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.Data == "body" {
				return m
			}
			a := map[string]string{}
			for _, attr := range t.Attr {
				if attr.Namespace == "" {
					a[strings.ToLower(attr.Key)] = attr.Val
				}
			}
			if t.Data == "link" {
				if strings.EqualFold(a["rel"], "canonical") && m.canonical == "" {
					m.canonical = a["href"]
				}
				if strings.EqualFold(a["rel"], "alternate") && a["type"] == "application/json+oembed" && m.endpoint == "" {
					m.endpoint = a["href"]
				}
			}
			if t.Data != "meta" {
				continue
			}
			key := strings.ToLower(a["property"])
			if key == "" {
				key = strings.ToLower(a["name"])
			}
			v := a["content"]
			switch key {
			case "og:title":
				m.title = cleanText(v, 300)
			case "og:description":
				m.description = cleanText(v, 1000)
			case "og:url":
				m.canonical = v
			case "og:image", "og:image:secure_url":
				if m.image == "" || key == "og:image:secure_url" {
					m.image = v
				}
			case "og:video", "og:video:url", "og:video:secure_url":
				if m.video == "" || key == "og:video:secure_url" {
					m.video = v
				}
			case "og:video:type":
				m.videoType = v
			case "og:video:width":
				m.videoWidth, _ = strconv.Atoi(v)
			case "og:video:height":
				m.videoHeight, _ = strconv.Atoi(v)
			}
		}
	}
}

// Additional provider restrictions compose with the existing public-address,
// redirect-count and header-redaction checks. Copying the client keeps shared
// resolver requests independent and never brings a cookie jar into a fetch.
func (r *Resolver) nativeProviderClient(allowed func(string) bool) *Resolver {
	resolver, client := *r, *r.http
	original := client.CheckRedirect
	client.Jar = nil
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return ErrUnavailable
		}
		if original != nil {
			if err := original(next, via); err != nil {
				return err
			}
		}
		if !allowed(next.URL.String()) {
			return ErrRestricted
		}
		return nil
	}
	resolver.http = &client
	return &resolver
}

func (r *Resolver) fetchNativeProviderPage(ctx context.Context, source nativeProvider, raw string) (nativeProviderMetadata, error) {
	client := r.nativeProviderClient(func(next string) bool { return sameNativeProvider(source, next) })
	res, err := client.fetch(ctx, raw, "text/html, application/xhtml+xml;q=0.9")
	if err != nil {
		return nativeProviderMetadata{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return nativeProviderMetadata{}, ErrRestricted
	}
	if res.StatusCode != http.StatusOK || res.Request == nil || !sameNativeProvider(source, res.Request.URL.String()) {
		return nativeProviderMetadata{}, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxPageBytes+1))
	if err != nil || len(data) > maxPageBytes || !isHTML(data) {
		return nativeProviderMetadata{}, ErrUnavailable
	}
	reader, err := charset.NewReader(bytes.NewReader(data), res.Header.Get("Content-Type"))
	if err != nil {
		return nativeProviderMetadata{}, ErrUnavailable
	}
	data, err = io.ReadAll(io.LimitReader(reader, maxPageBytes+1))
	if err != nil || len(data) > maxPageBytes {
		return nativeProviderMetadata{}, ErrLimit
	}
	m := readNativeProviderMetadata(data)
	if !sameNativeProvider(source, m.canonical) || strings.EqualFold(m.title, "Private GIF") {
		return nativeProviderMetadata{}, ErrRestricted
	}
	return m, nil
}

func nativeProviderOEmbed(source nativeProvider, m nativeProviderMetadata, raw string) string {
	endpoint := m.endpoint
	if source.name == "smugmug" {
		endpoint, _ = registryLookup()(raw)
	}
	u, ok := nativeProviderURL(endpoint)
	if !ok {
		return ""
	}
	if source.name == "droplr" && (u.Hostname() != "embed.droplr.com" || u.Path != "/oembed") || source.name == "smugmug" && (u.Hostname() != "api.smugmug.com" || u.Path != "/services/oembed/") {
		return ""
	}
	if !sameNativeProvider(source, u.Query().Get("url")) || !publicProviderQuery(u, 0) {
		return ""
	}
	u.RawQuery = url.Values{"url": {m.canonical}, "format": {"json"}}.Encode()
	return u.String()
}

func nativeProviderMedia(source nativeProvider, raw string) bool {
	u, ok := nativeProviderURL(raw)
	if !ok || len(u.RawQuery) > 8192 {
		return false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	host := strings.ToLower(u.Hostname())
	leaf := parts[len(parts)-1]
	ext := strings.ToLower(path.Ext(leaf))
	stem := strings.TrimSuffix(leaf, path.Ext(leaf))
	switch source.name {
	case "droplr":
		// The anonymous public image route may redirect to CloudFront-signed
		// delivery of that same drop; these response queries are not forwarded
		// to another host or exposed as the canonical source URL.
		query, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return false
		}
		for key, values := range query {
			if len(values) != 1 || !strings.Contains("|response-content-disposition|Expires|Key-Pair-Id|Signature|", "|"+key+"|") {
				return false
			}
		}
		if droplrHost.MatchString(host) {
			return source.kind == "image" && len(parts) == 2 && parts[0] == "i" && stem == source.id && strings.Contains("|.png|.jpg|.jpeg|.gif|.webp|", "|"+ext+"|")
		}
		return (host == "cdn-std.droplr.net" || host == "cdn-sec.droplr.net") && len(parts) == 3 && parts[0] == "files" && strings.HasPrefix(parts[1], "acc_") && decimalID.MatchString(strings.TrimPrefix(parts[1], "acc_")) && leaf == source.id
	case "imageshack":
		if len(source.id) < 4 {
			return false
		}
		shard, err := strconv.ParseUint(source.id[:2], 36, 16)
		return err == nil && u.RawQuery == "" && host == "imagizer.imageshack.com" && len(parts) == 4 && parts[0] == "v2" && parts[2] == strconv.FormatUint(shard, 10) && len(source.id) == len(stem)+3 && source.id[2:len(source.id)-1] == stem && strings.Contains("|.jpg|.jpeg|.png|.gif|.webp|", "|"+ext+"|")
	case "smugmug":
		if u.RawQuery != "" || (host != source.host && host != "photos.smugmug.com") || !strings.Contains("|.jpg|.jpeg|.png|.gif|.webp|", "|"+ext+"|") {
			return false
		}
		for _, part := range parts[:len(parts)-1] {
			if part == source.id {
				return true
			}
		}
	case "giphy-clips":
		if u.RawQuery != "" || !giphyMediaHost.MatchString(host) || parts[0] != "media" || leaf != "giphy.mp4" {
			return false
		}
		return len(parts) == 3 && parts[1] == source.id || len(parts) == 4 && strings.HasPrefix(parts[1], "v1.") && parts[2] == source.id
	}
	return false
}

// resolveNativeProvider is integrated before generic page/oEmbed resolution,
// so an unsuccessful or private page cannot be promoted by an alternate API.
func (r *Resolver) resolveNativeProvider(ctx context.Context, raw string) (Result, error) {
	source, ok := nativeProviderSource(raw)
	if !ok {
		return Result{}, ErrUnsupported
	}
	m, err := r.fetchNativeProviderPage(ctx, source, raw)
	if err != nil {
		return nativeProviderFailure(ctx, raw, m, err)
	}
	assetURL, width, height := m.image, m.videoWidth, m.videoHeight
	if source.name == "droplr" || source.name == "smugmug" {
		endpoint := nativeProviderOEmbed(source, m, raw)
		if endpoint == "" {
			return nativeProviderFailure(ctx, raw, m, ErrUnsupported)
		}
		u, _ := url.Parse(endpoint)
		client := r.nativeProviderClient(func(next string) bool {
			n, ok := nativeProviderURL(next)
			return ok && n.Host == u.Host && n.Path == u.Path && sameNativeProvider(source, n.Query().Get("url")) && publicProviderQuery(n, 0)
		})
		meta, _, fetchErr := client.fetchOEmbed(ctx, endpoint)
		if fetchErr != nil {
			return nativeProviderFailure(ctx, raw, m, fetchErr)
		}
		if source.kind == "image" {
			if meta.Type != oembed.TypePhoto {
				return nativeProviderFailure(ctx, raw, m, ErrUnsupported)
			}
			assetURL = meta.URL
		} else if meta.Type != oembed.TypeVideo {
			return nativeProviderFailure(ctx, raw, m, ErrUnsupported)
		}
		width, height = meta.Width, meta.Height
	}
	if source.kind == "video" {
		assetURL = m.video
		ct, _, _ := mime.ParseMediaType(m.videoType)
		if ct != "video/mp4" || width < 1 || width > 8192 || height < 1 || height > 8192 {
			return nativeProviderFailure(ctx, raw, m, ErrUnsupported)
		}
	}
	if !nativeProviderMedia(source, assetURL) {
		return nativeProviderFailure(ctx, raw, m, ErrRestricted)
	}
	client := r.nativeProviderClient(func(next string) bool { return nativeProviderMedia(source, next) })
	asset, err := client.ingest(ctx, Asset{URL: assetURL, Kind: source.kind}, "")
	if err != nil {
		return nativeProviderFailure(ctx, raw, m, err)
	}
	if source.kind == "video" {
		asset.Width, asset.Height = width, height
	}
	return Result{URL: raw, Provider: source.name, Title: m.title, Description: m.description, Assets: []Asset{asset}, CacheUntil: asset.ExpiresAt}, nil
}

func nativeProviderFailure(ctx context.Context, raw string, m nativeProviderMetadata, err error) (Result, error) {
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if errors.Is(err, ErrBusy) {
		return Result{}, err
	}
	return infoCard(raw, m.title, m.description, failureReason(err), time.Minute), nil
}
