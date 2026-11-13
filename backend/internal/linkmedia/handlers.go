package linkmedia

import (
	"net/url"
	"strings"
	"time"

	"ircord/backend/internal/mediafetch"
)

// Shared URL/extractor policy for explicitly enabled native social providers.
type providerHandler struct {
	name          string
	hosts         []string
	canonicalURL  func(*url.URL) (string, error)
	extractor     string
	photoFallback func(ytInfo) (Asset, error)
	finishExtract func(*Result)
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
