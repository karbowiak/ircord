package linkmedia

import (
	"context"
	"net/url"
	"path"
	"strings"
)

func directMediaFile(u *url.URL) bool {
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".gif", ".jpg", ".jpeg", ".png", ".webp", ".avif", ".bmp", ".mp4", ".webm", ".mov", ".m4v", ".ogv", ".mp3", ".m4a", ".ogg", ".wav", ".flac":
		return true
	}
	return false
}

// Minted IRCord file URLs carry deletion authority at their source. Recognize
// the exact route shape across origins without granting any destination access.
func fileCapabilityURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) != 5 || parts[0] != "" || parts[1] != "files" || parts[4] == "" {
		return false
	}
	return len(parts[2]) == 32 && len(parts[3]) == 48 &&
		strings.Trim(parts[2], "0123456789abcdef") == "" && // gitleaks:allow -- hexadecimal validation alphabet
		strings.Trim(parts[3], "0123456789abcdef") == "" // gitleaks:allow -- hexadecimal validation alphabet
}

// Verify a bounded prefix, then optionally copy ordinary media into the CDN.
// File capabilities always retain source delivery and its deletion checks.
func (r *Resolver) resolveFile(ctx context.Context, raw string) (Result, error) {
	asset, err := r.probeAsset(ctx, raw)
	if err != nil {
		return Result{}, err
	}
	if r.persistent != nil && !fileCapabilityURL(raw) {
		asset, err = r.deliver(ctx, asset)
		if err != nil {
			return Result{}, err
		}
	}
	return Result{URL: raw, Provider: "media", Assets: []Asset{asset}}, nil
}
