package linkmedia

import (
	"context"
	"errors"
	"time"

	"ircord/backend/internal/mediafetch"
)

const maxNativeAssets = 30
const maxNativeResultBytes int64 = 100 << 20

func (r *Resolver) deliverAll(ctx context.Context, result Result) (Result, error) {
	if len(result.Assets) > maxNativeAssets {
		return Result{}, ErrLimit
	}
	for _, asset := range result.Assets {
		// Native collections cannot accidentally publish documents as images.
		if asset.Kind != "image" && asset.Kind != "video" && asset.Kind != "audio" {
			return Result{}, ErrUnsupported
		}
	}
	result.Assets = append([]Asset(nil), result.Assets...)
	var copied []string
	remaining := maxNativeResultBytes
	for index, asset := range result.Assets {
		var delivered Asset
		var err error
		if remaining <= 0 {
			err = ErrLimit
		} else {
			delivered, err = r.deliver(ctx, asset, min(r.options.MaxFileBytes, remaining))
		}
		if err != nil {
			// Store assigns unique keys, so these belong only to this failed result.
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			err = errors.Join(err, r.discardObjects(cleanup, copied))
			return Result{}, err
		}
		if delivered.ObjectKey != "" && delivered.ObjectKey != asset.ObjectKey {
			copied = append(copied, delivered.ObjectKey)
		}
		result.Assets[index] = delivered
		remaining -= delivered.Size
	}
	return result, nil
}

func (r *Resolver) deliver(ctx context.Context, asset Asset, limit int64) (Asset, error) {
	if _, err := mediafetch.URL(asset.URL); err != nil {
		return Asset{}, err
	}
	// Retained connector helpers also cannot restore third-party delivery.
	return r.ingestLimited(ctx, asset, "", limit)
}
