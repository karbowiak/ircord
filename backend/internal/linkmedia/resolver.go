package linkmedia

import (
	"context"
	"net/http"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/iframely"
	"ircord/backend/internal/mediafetch"
)

type ResolverOptions struct {
	// Reddit is the only active social connector. Its sources must pass the
	// same validated CDN ingest as direct files before becoming native assets.
	Reddit Connector
	// Retained for source compatibility; native resolution never invokes these.
	Instagram    Connector
	Downloader   Connector
	Iframely     *iframely.Client
	CDN          *cdn.Client
	HTTPClient   *http.Client // retained for compatibility; native I/O always uses mediafetch.Client
	MaxFileBytes int64
	Timeout      time.Duration
}

type Resolver struct {
	options    ResolverOptions
	http       *http.Client
	cache      *resolutionCache
	persistent *publicCache
}

func NewResolver(o ResolverOptions) *Resolver {
	if o.Timeout <= 0 || o.Timeout > 75*time.Second {
		o.Timeout = 75 * time.Second
	}
	if o.MaxFileBytes <= 0 || o.MaxFileBytes > 100<<20 {
		o.MaxFileBytes = 50 << 20
	}
	return &Resolver{options: o, http: nativeHTTPClient(), cache: &resolutionCache{}}
}

// Resolve shares a single bounded, durable native path with every API caller.
func (r *Resolver) Resolve(ctx context.Context, raw string) (Result, error) {
	return r.ResolveSource(ctx, raw)
}

func (r *Resolver) resolveNative(ctx context.Context, raw string) (Result, error) {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return Result{}, err
	}
	if fileCapabilityURL(raw) {
		return r.resolveFile(ctx, raw)
	}
	if _, err := redditID(raw); err == nil && r.options.Reddit != nil {
		return r.resolveReddit(ctx, raw)
	}
	if directMediaFile(u) {
		return r.resolveFile(ctx, raw)
	}
	return r.resolvePage(ctx, raw)
}

// ResolveSource receives public URLs only. The API must authorize the message
// before and after this potentially slow operation, including durable hits.
func (r *Resolver) ResolveSource(ctx context.Context, raw string) (Result, error) {
	if _, err := mediafetch.URL(raw); err != nil {
		return Result{}, err
	}
	result, err := r.cache.get(ctx, resolutionVersion(raw)+":"+resolutionKey(raw), r.options.Timeout, func(work context.Context) (Result, error) {
		if r.persistent != nil {
			if cached, ok := r.loadPublicResult(work, raw); ok {
				return cached, nil
			}
		}
		value, err := r.resolveNative(work, raw)
		if err == nil && r.persistent != nil && storedResult(value) {
			if saveErr := r.savePublicResult(work, raw, value); saveErr != nil {
				value.CacheUntil = time.Now().Add(time.Minute)
			}
		}
		return value, err
	})
	result.URL = raw
	return result, err
}
