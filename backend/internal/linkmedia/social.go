package linkmedia

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"ircord/backend/internal/mediafetch"
)

// NativeSocialSource grants only routing authority, never media or account
// access. The API still validates every stored URL and message authorization.
func NativeSocialSource(raw, provider string) bool {
	_, name, _, err := socialSource(raw)
	return err == nil && name == provider
}

func socialQuery(u *url.URL) bool {
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(u.RawQuery) > 4096 || !publicProviderQuery(u, 0) {
		return false
	}
	for key, values := range q {
		if len(values) != 1 {
			return false
		}
		switch strings.ToLower(key) {
		case "resourcekey", "sessionid", "session", "cookie", "cookies", "accesskey":
			return false
		}
	}
	return true
}

func socialSource(raw string) (canonical, provider, id string, err error) {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return "", "", "", err
	}
	if !socialQuery(u) {
		return "", "", "", ErrRestricted
	}
	switch u.Hostname() {
	case "instagram.com", "www.instagram.com":
		canonical, id, err = instagramURL(raw)
		return canonical, "instagram", id, err
	case "tiktok.com", "www.tiktok.com", "m.tiktok.com":
		canonical, err = tiktokURL(u)
		if err != nil || !tiktokPath.MatchString(u.Path) {
			return "", "", "", ErrUnsupported
		}
		p := strings.Split(strings.Trim(u.Path, "/"), "/")
		return canonical, "tiktok", p[2], nil
	}
	return "", "", "", ErrUnsupported
}

func socialPageSource(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "instagram.com", "www.instagram.com", "tiktok.com", "www.tiktok.com", "m.tiktok.com", "vm.tiktok.com", "vt.tiktok.com":
		return true
	}
	return false
}

func sameSocialPost(raw, provider, id string) bool {
	_, name, found, err := socialSource(raw)
	return err == nil && name == provider && id == found
}

func socialMediaURL(raw, provider string) bool {
	u, err := mediafetch.URL(raw)
	if err != nil || u.Scheme != "https" || u.RawPath != "" || u.Port() != "" && u.Port() != "443" || fileCapabilityURL(raw) {
		return false
	}
	host := strings.ToLower(u.Hostname())
	domains := []string{"cdninstagram.com", "fbcdn.net"}
	if provider == "tiktok" {
		domains = []string{"tiktok.com", "tiktokcdn.com", "tiktokcdn-us.com", "tiktokv.com", "byteoversea.com", "ibytedtos.com", "ibyteimg.com", "muscdn.com"}
	}
	for _, domain := range domains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func validateSocialResult(value Result, provider, id string) error {
	if len(value.Assets) > maxNativeAssets {
		return ErrLimit
	}
	if value.Provider != provider || value.ID != id || value.Embed != nil || value.Preview != nil ||
		!sameSocialPost(value.URL, provider, id) || len(value.Assets) < 1 || len(value.Assets) > maxNativeAssets {
		return ErrUnavailable
	}
	for _, asset := range value.Assets {
		if asset.Kind != "image" && asset.Kind != "video" || !socialMediaURL(asset.URL, provider) {
			return ErrUnavailable
		}
	}
	return nil
}

func (r *Resolver) resolveSocial(ctx context.Context, raw string) (Result, error) {
	canonical, provider, id, err := socialSource(raw)
	if err != nil {
		reason := "unsupported_type"
		if errors.Is(err, ErrRestricted) {
			reason = "restricted"
		}
		return infoCard(raw, "", "", reason, restrictedCacheTTL), nil
	}
	var value Result
	err = ErrUnavailable
	if r.options.Downloader != nil {
		value, err = r.options.Downloader.Resolve(ctx, canonical)
		if err == nil {
			err = validateSocialResult(value, provider, id)
		}
	}
	// The public embed parser repairs missing photo/carousel metadata only. It
	// never uses account cookies, a Graph access token or a thumbnail substitute.
	if err != nil && provider == "instagram" && r.options.Instagram != nil && !errors.Is(err, ErrAgeRestricted) && !errors.Is(err, ErrLimit) {
		primaryErr := err
		value, err = r.options.Instagram.Resolve(ctx, canonical)
		if err == nil {
			err = validateSocialResult(value, provider, id)
		}
		if err != nil && errors.Is(primaryErr, ErrRestricted) {
			err = primaryErr
		}
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if err == nil {
		value, err = r.deliverAll(ctx, value)
		if err == nil {
			value.URL = raw
			value.Title = cleanText(value.Title, 300)
			value.Description = cleanText(value.Description, 4000)
			value.Author = cleanText(value.Author, 200)
			for i := range value.Assets {
				// Temporary signed source URLs and request headers have completed
				// their purpose. Only validated 90-day CDN URLs are retained.
				value.Assets[i].SourceURL = ""
				value.Assets[i].SourceExpiresAt = time.Time{}
				value.Assets[i].Headers = nil
			}
			return value, nil
		}
	}
	if errors.Is(err, ErrBusy) {
		return Result{}, err
	}
	reason := failureReason(err)
	if errors.Is(err, ErrAgeRestricted) {
		reason = "restricted"
	}
	result := infoCard(raw, "", "", reason, time.Minute)
	result.Provider, result.ID = provider, id
	return result, nil
}
