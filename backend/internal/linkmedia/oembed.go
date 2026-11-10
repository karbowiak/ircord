package linkmedia

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/artyom/oembed"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"ircord/backend/internal/mediafetch"
)

const maxPageBytes = 1 << 20
const maxOEmbedBytes = 256 << 10
const maxWidgetBytes = 192 << 10

// Official oembed.com/providers.json, vendored 2026-11-09. This complete
// snapshot is pinned in source, never downloaded or refreshed at runtime.
// Source registry: https://github.com/iamcal/oembed (MIT).
// The artyom library is used only for parsers and registry matching; it never
// performs HTTP requests. All actual I/O uses the public-only mediafetch client.
//
//go:embed oembed_providers.json
var providerSnapshot []byte

var registryLookup = sync.OnceValue(func() oembed.LookupFunc {
	lookup, err := oembed.Providers(bytes.NewReader(providerSnapshot))
	if err != nil {
		return func(string) (string, bool) { return "", false }
	}
	return lookup
})

// RegistrySnapshot identifies the exact vendored registry without network I/O.
func RegistrySnapshot() string {
	sum := sha256.Sum256(providerSnapshot)
	return hex.EncodeToString(sum[:])
}

func cleanText(text string, limit int) string {
	text = strings.Join(strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
	runes := []rune(text)
	if len(runes) > limit {
		text = string(runes[:limit])
	}
	return text
}

func infoCard(raw, title, description, reason string, ttl time.Duration) Result {
	u, _ := url.Parse(raw)
	if title == "" {
		title = u.Hostname()
	}
	until := time.Now().Add(ttl)
	return Result{URL: raw, Provider: "page", Title: cleanText(title, 300), Description: cleanText(description, 1000), CacheUntil: until,
		Preview: &Preview{Kind: "card", Site: u.Hostname(), Title: cleanText(title, 300), Description: cleanText(description, 1000), Reason: reason, ExpiresAt: until}}
}

func (r *Resolver) resolvePage(ctx context.Context, raw string) (Result, error) {
	if name := oembedAppFamily(raw); name != "" {
		return r.resolveOEmbedApp(ctx, raw, name)
	}
	if NativeProviderSource(raw) {
		return r.resolveNativeProvider(ctx, raw)
	}
	if appFrameSource(raw) {
		return r.resolveAppFramePage(ctx, raw)
	}
	if googlePageSource(raw) {
		return r.resolveGooglePage(ctx, raw)
	}
	title, description := "", ""
	var endpoints []string
	if endpoint := providerEndpoint(raw); endpoint != "" {
		endpoints = appendEndpoint(endpoints, endpoint)
	}
	var metadata pageMetadata
	pageAvailable := false
	reason := "no_oembed"
	page, err := r.fetch(ctx, raw, "text/html, application/xhtml+xml;q=0.9, */*;q=0.1")
	if err != nil {
		reason = failureReason(err)
	} else {
		defer page.Body.Close()
		if page.StatusCode == http.StatusOK {
			base := page.Request.URL
			for _, header := range page.Header.Values("Link") {
				for _, entry := range splitLinks(header) {
					if endpoint := linkEndpoint(entry, base); endpoint != "" {
						endpoints = appendEndpoint(endpoints, endpoint)
					}
				}
			}
			data, readErr := io.ReadAll(io.LimitReader(page.Body, maxPageBytes+1))
			if readErr != nil || len(data) > maxPageBytes {
				reason = "fetch_failed"
			} else if isHTML(data) {
				reader, decodeErr := charset.NewReader(bytes.NewReader(data), page.Header.Get("Content-Type"))
				if decodeErr == nil {
					utf8, readErr := io.ReadAll(io.LimitReader(reader, maxPageBytes+1))
					if readErr == nil && len(utf8) <= maxPageBytes {
						title, description = pageText(utf8)
						metadata = readPageMetadata(utf8)
						pageAvailable = true
						if metadata.title != "" {
							title = metadata.title
						}
						if metadata.description != "" {
							description = metadata.description
						}
						if endpoint, ok, discoverErr := oembed.Discover(bytes.NewReader(utf8)); discoverErr == nil && ok {
							if target := absoluteEndpoint(endpoint, base); target != "" {
								endpoints = appendEndpoint(endpoints, target)
							}
						}
					}
				}
			} else {
				reason = "unsupported_type"
			}
		} else if page.StatusCode == 401 || page.StatusCode == 403 {
			reason = "restricted"
		} else {
			reason = "fetch_failed"
		}
	}
	// Registry fallback is a local lookup of the official snapshot, never an
	// endpoint guess or an extraction call to a custom provider.
	if endpoint, ok := registryLookup()(raw); ok {
		// Reserve the final attempt for registry fallback, even when a page
		// advertises several broken discovery links.
		if len(endpoints) > 2 {
			endpoints = endpoints[:2]
		}
		endpoints = appendEndpoint(endpoints, endpoint)
	}
	var genericFallback *Result
	for _, endpoint := range endpoints {
		meta, ttl, fetchErr := r.fetchOEmbed(ctx, endpoint)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if fetchErr != nil {
			reason = failureReason(fetchErr)
			continue
		}
		value, adaptErr := r.oembedResult(ctx, raw, meta, ttl)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if errors.Is(adaptErr, ErrBusy) {
			return Result{}, adaptErr
		}
		if adaptErr == nil {
			if len(value.Assets) > 0 || value.Preview != nil && providerMarkup(raw, value.Preview.HTML) != "" {
				return value, nil
			}
			// Unknown rich markup is only a fallback: a provider page may
			// supply a native image or a controlled player instead.
			if genericFallback == nil {
				genericFallback = &value
			}
			continue
		}
		reason = failureReason(adaptErr)
		if meta.Title != "" {
			title = meta.Title
		}
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if pageAvailable {
		value, ok, providerErr := r.pageProviderResult(ctx, raw, metadata)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if ok || errors.Is(providerErr, ErrBusy) {
			return value, providerErr
		}
		if providerErr != nil {
			reason = failureReason(providerErr)
		}
	}
	ttl := 24 * time.Hour
	if reason == "fetch_failed" || reason == "storage_unavailable" {
		ttl = time.Minute
	}
	if reason == "restricted" {
		ttl = 5 * time.Minute
	}
	if genericFallback != nil {
		// A temporarily unavailable native result should be retried soon,
		// even when the provider also returned generic rich markup.
		until := time.Now().Add(ttl)
		if until.Before(genericFallback.CacheUntil) {
			genericFallback.CacheUntil = until
			if genericFallback.Preview != nil {
				genericFallback.Preview.ExpiresAt = until
			}
		}
		return *genericFallback, nil
	}
	value := infoCard(raw, title, description, reason, ttl)
	if pageAvailable && metadata.image != "" {
		u, _ := url.Parse(raw)
		if image := metadataURL(metadata.image, u); image != "" {
			asset, ingestErr := r.ingest(ctx, Asset{URL: image, Kind: "image"}, "")
			if errors.Is(ingestErr, ErrBusy) {
				return Result{}, ingestErr
			}
			if ingestErr == nil {
				value.Preview.Image, value.Preview.ImageWidth, value.Preview.ImageHeight = asset.URL, asset.Width, asset.Height
				value.Preview.ImageAsset = &asset
			}
		}
	}
	return value, nil
}

func isHTML(data []byte) bool {
	ct := strings.SplitN(http.DetectContentType(data), ";", 2)[0]
	return ct == "text/html"
}

// Read only the HTML title and standard description, never OG/Twitter fields,
// scripts, arbitrary DOM content, icons, or third-party image URLs.
func pageText(data []byte) (string, string) {
	z := html.NewTokenizer(bytes.NewReader(data))
	z.SetMaxBuf(maxPageBytes)
	var title, description string
	inTitle := false
	for {
		switch z.Next() {
		case html.ErrorToken:
			return cleanText(title, 300), cleanText(description, 1000)
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			if token.Data == "body" {
				return cleanText(title, 300), cleanText(description, 1000)
			}
			if token.Data == "title" {
				inTitle = true
			}
			if token.Data == "meta" && description == "" {
				name, content := "", ""
				for _, attr := range token.Attr {
					if attr.Key == "name" {
						name = attr.Val
					}
					if attr.Key == "content" {
						content = attr.Val
					}
				}
				if strings.EqualFold(name, "description") {
					description = content
				}
			}
		case html.EndTagToken:
			if token := z.Token(); token.Data == "title" {
				inTitle = false
			}
		case html.TextToken:
			if inTitle && len(title) < 2048 {
				title += string(z.Text())
			}
		}
	}
}

func appendEndpoint(all []string, endpoint string) []string {
	if len(all) >= 3 {
		return all
	}
	for _, existing := range all {
		if existing == endpoint {
			return all
		}
	}
	return append(all, endpoint)
}

func absoluteEndpoint(raw string, base *url.URL) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u = base.ResolveReference(u)
	if _, err = mediafetch.URL(u.String()); err != nil || fileCapabilityURL(u.String()) {
		return ""
	}
	return u.String()
}

func splitLinks(value string) []string {
	var out []string
	quoted, angle, escaped := false, false, false
	start := 0
	for i, c := range value {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && quoted {
			escaped = true
			continue
		}
		if c == '"' {
			quoted = !quoted
		}
		if !quoted {
			if c == '<' {
				angle = true
			}
			if c == '>' {
				angle = false
			}
		}
		if c == ',' && !quoted && !angle {
			out = append(out, strings.TrimSpace(value[start:i]))
			start = i + 1
		}
	}
	return append(out, strings.TrimSpace(value[start:]))
}

func linkEndpoint(value string, base *url.URL) string {
	if !strings.HasPrefix(value, "<") {
		return ""
	}
	end := strings.IndexByte(value, '>')
	if end < 1 {
		return ""
	}
	_, params, err := mime.ParseMediaType("link" + value[end+1:])
	if err != nil {
		return ""
	}
	ct := strings.ToLower(params["type"])
	if ct != "application/json+oembed" && ct != "text/xml+oembed" {
		return ""
	}
	alternate := false
	for _, rel := range strings.Fields(params["rel"]) {
		if strings.EqualFold(rel, "alternate") {
			alternate = true
		}
	}
	if !alternate {
		return ""
	}
	return absoluteEndpoint(value[1:end], base)
}

func (r *Resolver) fetchOEmbed(ctx context.Context, raw string) (*oembed.Metadata, time.Duration, error) {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return nil, 0, ErrUnavailable
	}
	q := u.Query()
	q.Set("maxwidth", "960")
	q.Set("maxheight", "720")
	u.RawQuery = q.Encode()
	res, err := r.fetch(ctx, u.String(), "application/json, text/xml;q=0.9, application/xml;q=0.9")
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	// Reviewed app metadata must originate at the exact official endpoint.
	// A redirect to a login page, another publisher or another endpoint cannot
	// supply player authority, even when it returns otherwise valid JSON.
	if oembedAppEndpoint(u) && (res.Request == nil || res.Request.URL == nil || res.Request.URL.String() != u.String()) {
		return nil, 0, ErrRestricted
	}
	if res.StatusCode != http.StatusOK {
		if res.StatusCode == 401 || res.StatusCode == 403 {
			return nil, 0, ErrRestricted
		}
		return nil, 0, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxOEmbedBytes+1))
	if err != nil || len(data) > maxOEmbedBytes {
		return nil, 0, ErrUnavailable
	}
	ct, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	var meta *oembed.Metadata
	if ct == "text/xml" || ct == "application/xml" || ct == "text/xml+oembed" {
		meta, err = oembed.FromXML(bytes.NewReader(data))
	} else if ct == "application/json" || ct == "application/json+oembed" {
		meta, err = oembed.FromJSON(bytes.NewReader(data))
	} else {
		return nil, 0, ErrUnavailable
	}
	if err != nil || meta == nil {
		return nil, 0, ErrUnavailable
	}
	ttl := 24 * time.Hour
	var extra struct {
		CacheAge json.Number `json:"cache_age"`
	}
	if ct == "application/json" || ct == "application/json+oembed" {
		if json.Unmarshal(data, &extra) == nil && extra.CacheAge != "" {
			if seconds, parseErr := extra.CacheAge.Int64(); parseErr == nil && seconds >= 0 && seconds < 86400 {
				ttl = time.Duration(seconds) * time.Second
			}
		}
	}
	// A minimum retry interval bounds repeated uncacheable provider responses.
	if ttl < time.Minute {
		ttl = time.Minute
	}
	return meta, ttl, nil
}

func (r *Resolver) oembedResult(ctx context.Context, raw string, meta *oembed.Metadata, ttl time.Duration) (Result, error) {
	u, _ := url.Parse(raw)
	p := &Preview{Kind: "card", Site: cleanText(meta.Provider, 120), Title: cleanText(meta.Title, 300), ExpiresAt: time.Now().Add(ttl)}
	if p.Site == "" {
		p.Site = u.Hostname()
	}
	if p.Title == "" {
		p.Title = p.Site
	}
	value := Result{URL: raw, Provider: "oembed", Title: p.Title, Author: cleanText(meta.AuthorName, 200), CacheUntil: p.ExpiresAt, Preview: p}
	switch meta.Type {
	case oembed.TypePhoto:
		if meta.URL == "" {
			return Result{}, ErrUnavailable
		}
		asset, err := r.ingest(ctx, Asset{URL: meta.URL, Kind: "image"}, "")
		if err != nil {
			return Result{}, err
		}
		value.Assets = []Asset{asset}
		value.Preview = nil
	case oembed.TypeVideo, oembed.TypeRich:
		if strings.TrimSpace(meta.HTML) == "" || len(meta.HTML) > maxWidgetBytes {
			return Result{}, ErrUnavailable
		}
		p.Kind, p.HTML = "widget", meta.HTML
		if markup := providerMarkup(raw, meta.HTML); markup != "" {
			p.HTML = markup
		}
		if meta.Width > 0 && meta.Width <= 8192 {
			p.Width = meta.Width
		}
		if meta.Height > 0 && meta.Height <= 8192 {
			p.Height = meta.Height
		}
	case oembed.TypeLink:
	default:
		return Result{}, ErrUnsupported
	}
	if value.Preview != nil && meta.Thumbnail != "" {
		asset, err := r.ingest(ctx, Asset{URL: meta.Thumbnail, Kind: "image"}, "")
		if errors.Is(err, ErrBusy) {
			return Result{}, err
		}
		if err == nil {
			p.Image, p.ImageWidth, p.ImageHeight = asset.URL, asset.Width, asset.Height
			p.ImageAsset = &asset
		} else {
			// Retain usable oEmbed markup, but retry failed thumbnail delivery soon.
			p.Reason = failureReason(err)
			if p.Reason == "fetch_failed" || p.Reason == "storage_unavailable" {
				value.CacheUntil = time.Now().Add(time.Minute)
				p.ExpiresAt = value.CacheUntil
			}
		}
	}
	return value, nil
}
