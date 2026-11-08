package linkmedia

import (
	"context"
	"net/url"
	"strings"
	"time"

	"ircord/backend/internal/iframely"
	"ircord/backend/internal/mediafetch"
)

// Each provider owns its URL rules, extraction chain and optional widget or
// yt-dlp adaptations in its own file. Add new providers to this registry;
// matching and the final Iframely fallback stay shared.
type providerHandler struct {
	name          string
	hosts         []string
	canonicalURL  func(*url.URL) (string, error)
	connectors    func(ResolverOptions) []Connector
	extractor     string
	photoFallback func(ytInfo) (Asset, error)
	finishExtract func(*Result)
	widget        func(*url.URL, iframely.Response) (Result, bool)
	sourceExpiry  func(*url.URL, url.Values) time.Time
}

var providerHandlers = []providerHandler{
	redditHandler,
	instagramHandler,
	tiktokHandler,
}

func handlerFor(u *url.URL) *providerHandler {
	for index := range providerHandlers {
		handler := &providerHandlers[index]
		for _, host := range handler.hosts {
			if strings.EqualFold(u.Hostname(), host) {
				return handler
			}
		}
	}
	return nil
}

func providerTarget(raw string) (string, *providerHandler, error) {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return "", nil, err
	}
	u.Scheme = "https"
	handler := handlerFor(u)
	if handler == nil {
		return "", nil, ErrUnsupported
	}
	if handler.canonicalURL == nil {
		return u.String(), handler, nil
	}
	canonical, err := handler.canonicalURL(u)
	return canonical, handler, err
}

func extractionTarget(raw string) (string, *providerHandler, error) {
	canonical, handler, err := providerTarget(raw)
	if err == nil && handler.extractor == "" {
		return "", nil, ErrUnsupported
	}
	return canonical, handler, err
}

func extractionURL(raw string) (string, string, error) {
	canonical, handler, err := extractionTarget(raw)
	if handler == nil {
		return canonical, "", err
	}
	return canonical, handler.name, err
}

func (r *Resolver) resolveConnectors(ctx context.Context, raw string, connectors []Connector) (Result, error) {
	var lastErr error = ErrUnavailable
	for _, connector := range connectors {
		if connector == nil {
			continue
		}
		result, err := connector.Resolve(ctx, raw)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if err != nil {
			lastErr = err
			continue
		}
		if len(result.Assets) == 0 && result.Description == "" {
			lastErr = ErrUnavailable
			continue
		}
		// Another extractor would yield the same unservable media. The caller
		// falls back to Iframely when source delivery fails.
		return r.deliverAll(ctx, result)
	}
	return Result{}, lastErr
}
