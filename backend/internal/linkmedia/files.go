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

// Verify a bounded prefix, then let the browser use the original file URL.
func (r *Resolver) resolveFile(ctx context.Context, raw string) (Result, error) {
	asset, err := r.probeAsset(ctx, raw)
	if err != nil {
		return Result{}, err
	}
	return Result{URL: raw, Provider: "media", Assets: []Asset{asset}}, nil
}
