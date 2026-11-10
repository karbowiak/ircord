package linkmedia

import (
	"context"
	"net/http"
	"strings"

	"ircord/backend/internal/mediafetch"
)

// Extractor headers may describe a public CDN request, never an account
// session. Authorization/account cookies are excluded; reviewed anonymous
// TikTok cookie names apply only to the exact extraction origin, never a
// redirected origin. Every redirect is still SSRF-checked.
func (r *Resolver) fetchExtractionAsset(ctx context.Context, asset Asset) (*http.Response, error) {
	provider := ""
	if socialMediaURL(asset.URL, "instagram") {
		provider = "instagram"
	} else if socialMediaURL(asset.URL, "tiktok") {
		provider = "tiktok"
	}
	if provider == "" || len(asset.Headers) == 0 {
		return r.fetch(ctx, asset.URL, "*/*")
	}
	u, err := mediafetch.URL(asset.URL)
	if err != nil || fileCapabilityURL(asset.URL) {
		return nil, ErrRestricted
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "IRCord-LinkPreview/1.0")
	if ua := asset.Headers.Get("User-Agent"); len(ua) > 0 && len(ua) <= 512 && !strings.ContainsAny(ua, "\x00\r\n") {
		req.Header.Set("User-Agent", ua)
	}
	base := "https://www." + provider + ".com"
	if ref := asset.Headers.Get("Referer"); ref == base || ref == base+"/" {
		req.Header.Set("Referer", base+"/")
	} else if canonical, name, _, err := socialSource(ref); err == nil && name == provider {
		req.Header.Set("Referer", canonical)
	}
	if asset.Headers.Get("Origin") == base {
		req.Header.Set("Origin", base)
	}
	if provider == "tiktok" && asset.anonymousCookieOrigin == u.Scheme+"://"+u.Host {
		pairs := []string{}
		for _, pair := range strings.Split(asset.Headers.Get("Cookie"), ";") {
			pair = strings.TrimSpace(pair)
			name, _, ok := strings.Cut(pair, "=")
			if ok && anonymousTikTokCookie(name) && len(pair) <= 4096 && !strings.ContainsAny(pair, "\x00\r\n") {
				pairs = append(pairs, pair)
			}
		}
		if len(pairs) <= 8 && len(pairs) > 0 {
			req.Header.Set("Cookie", strings.Join(pairs, "; "))
		}
	}
	client := *r.http
	prior := client.CheckRedirect
	client.Jar = nil
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return ErrLimit
		}
		if _, err := mediafetch.URL(next.URL.String()); err != nil || fileCapabilityURL(next.URL.String()) || next.URL.Scheme != "https" {
			return ErrRestricted
		}
		if next.URL.Scheme != u.Scheme || next.URL.Host != u.Host {
			for key := range next.Header {
				next.Header.Del(key)
			}
		}
		next.Header.Set("Accept-Encoding", "identity")
		if prior != nil {
			return prior(next, via)
		}
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, ErrUnavailable
	}
	if encoding := strings.TrimSpace(res.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		res.Body.Close()
		return nil, ErrLimit
	}
	return res, nil
}
