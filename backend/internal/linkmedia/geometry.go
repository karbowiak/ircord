package linkmedia

import (
	"context"
	"encoding/json"
	"image"
	"strings"
	"time"

	"ircord/backend/internal/mediafetch"
)

const MaxGeometryURLs = 256
const maxGeometryManifestBytes = 16 << 10

// ImageGeometry contains sizing advice only, never a media capability.
type ImageGeometry struct {
	Width, Height int
}

// CachedImageGeometry reads at most one bounded batch of retained manifests.
// It never resolves a URL, downloads bytes, probes storage, or renews expiry.
// Missing metadata and storage failures are expected and simply omit hints.
func (r *Resolver) CachedImageGeometry(ctx context.Context, sources []string) map[string]ImageGeometry {
	out := map[string]ImageGeometry{}
	if r == nil || r.persistent == nil || r.options.CDN == nil || ctx.Err() != nil {
		return out
	}
	keys := map[string]string{}
	args := []any{r.persistent.namespace, time.Now().UnixNano(), maxGeometryManifestBytes}
	for _, raw := range sources {
		if len(keys) >= MaxGeometryURLs {
			break
		}
		if _, err := mediafetch.URL(raw); err != nil || fileCapabilityURL(raw) {
			continue
		}
		key := publicResultKey(raw)
		if _, found := keys[key]; !found {
			keys[key] = raw
			args = append(args, key)
		}
	}
	if len(keys) == 0 {
		return out
	}
	// This optional decoration must not wait behind unrelated history writes.
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	rows, err := r.persistent.db.QueryContext(ctx, `SELECT cache_key,payload FROM public_media_results
		WHERE namespace=? AND expires_ns>? AND length(payload)<=? AND cache_key IN (`+placeholders+`)`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	now := time.Now()
	for rows.Next() {
		var key string
		var payload []byte
		if rows.Scan(&key, &payload) != nil {
			return map[string]ImageGeometry{}
		}
		var value Result
		if len(payload) > maxGeometryManifestBytes || json.Unmarshal(payload, &value) != nil || value.URL != keys[key] ||
			value.Embed != nil || value.Preview != nil || len(value.Assets) != 1 || !storedResult(value) {
			continue
		}
		asset := value.Assets[0]
		if asset.Kind != "image" || allowedKind(asset.ContentType) != "image" ||
			!imageBounds(image.Config{Width: asset.Width, Height: asset.Height}) || !asset.ExpiresAt.After(now) ||
			!r.IsDeliveryURL(asset.URL) || r.options.CDN.URL(asset.ObjectKey) != asset.URL {
			continue
		}
		out[value.URL] = ImageGeometry{Width: asset.Width, Height: asset.Height}
	}
	if rows.Err() != nil {
		return map[string]ImageGeometry{}
	}
	return out
}
