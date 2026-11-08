package linkmedia

import (
	"context"
	"errors"
	"net/http"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/iframely"
	"ircord/backend/internal/mediafetch"
)

type ResolverOptions struct {
	Reddit       Connector
	Instagram    Connector
	Downloader   Connector // normally NewYTDLP; nil disables subprocess extraction
	Iframely     *iframely.Client
	CDN          *cdn.Client // optional; used only if source delivery needs a stored copy
	HTTPClient   *http.Client
	MaxFileBytes int64
	Timeout      time.Duration
}

type Resolver struct {
	options ResolverOptions
	http    *http.Client
	cache   *resolutionCache
}

func NewResolver(o ResolverOptions) *Resolver {
	if o.Timeout <= 0 {
		o.Timeout = 75 * time.Second
	}
	if o.MaxFileBytes <= 0 {
		o.MaxFileBytes = 50 << 20
	}
	return &Resolver{options: o, http: httpClient(o.HTTPClient), cache: &resolutionCache{}}
}

// Resolve routes direct files and supported providers through their handlers.
// Unhandled URLs and failed source extraction always fall back to Iframely.
func (r *Resolver) Resolve(ctx context.Context, raw string) (Result, error) {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, r.options.Timeout)
	defer cancel()
	raw = u.String()
	if directMediaFile(u) {
		if result, err := r.resolveFile(ctx, raw); err == nil {
			return result, nil
		}
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
	}
	var directErr error = ErrUnavailable
	if handler := handlerFor(u); handler != nil && handler.connectors != nil {
		result, err := r.resolveConnectors(ctx, raw, handler.connectors(r.options))
		if err == nil {
			return result, nil
		}
		directErr = err
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if r.options.Iframely == nil {
		return Result{}, directErr
	}
	result, err := r.ResolveWidget(ctx, raw)
	if err != nil {
		return Result{}, errors.Join(directErr, err)
	}
	return result, nil
}

// ResolveSource resolves a visible message. Media a browser can load directly
// is served from its source; only media that needs extraction headers (TikTok's
// cookie-bound files, for one) is copied into the CDN when one is configured.
func (r *Resolver) ResolveSource(ctx context.Context, raw string) (Result, error) {
	return r.cache.get(ctx, "source:"+resolutionKey(raw), r.options.Timeout, func(ctx context.Context) (Result, error) {
		return r.Resolve(ctx, raw)
	})
}
