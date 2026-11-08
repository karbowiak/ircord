package linkmedia

import (
	"context"
	"errors"
	"strings"
	"time"

	"ircord/backend/internal/mediafetch"
)

func (r *Resolver) deliverAll(ctx context.Context, result Result) (Result, error) {
	result.Assets = append([]Asset(nil), result.Assets...)
	var copied []string
	for index, asset := range result.Assets {
		delivered, err := r.deliver(ctx, asset)
		if err != nil {
			// Store assigns unique keys, so these belong only to this failed result.
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			for _, key := range copied {
				err = errors.Join(err, r.options.CDN.Delete(cleanup, key))
			}
			return Result{}, err
		}
		if delivered.ObjectKey != "" && delivered.ObjectKey != asset.ObjectKey {
			copied = append(copied, delivered.ObjectKey)
		}
		result.Assets[index] = delivered
	}
	return result, nil
}

func (r *Resolver) deliver(ctx context.Context, asset Asset) (Asset, error) {
	if _, err := mediafetch.URL(asset.URL); err != nil {
		return Asset{}, err
	}
	// Browser delivery must work without backend authorization or cookies.
	if asset.Headers.Get("Authorization") == "" && asset.Headers.Get("Cookie") == "" {
		probed, err := r.probeAsset(ctx, asset.URL)
		if err == nil && strings.HasPrefix(probed.ContentType, asset.Kind+"/") {
			asset.ContentType, asset.Size = probed.ContentType, probed.Size
			// Known dimensions let the client reserve the media's space before it loads.
			if asset.Width == 0 || asset.Height == 0 {
				asset.Width, asset.Height = probed.Width, probed.Height
			}
			asset.Headers = nil
			return asset, nil
		}
	}
	if r.options.CDN != nil {
		return r.Store(ctx, asset)
	}
	return Asset{}, ErrUnavailable
}
