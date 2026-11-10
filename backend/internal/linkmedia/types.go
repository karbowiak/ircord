// Package linkmedia resolves public links into native media or isolated widgets.
// Optional CDN copies have a separate durable cache, independent of chat history.
package linkmedia

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"ircord/backend/internal/iframely"
)

var (
	ErrUnsupported   = errors.New("unsupported media link")
	ErrUnavailable   = errors.New("direct media unavailable")
	ErrLimit         = errors.New("media exceeds limit")
	ErrRestricted    = errors.New("post is not publicly accessible")
	ErrAgeRestricted = errors.New("post is age restricted")
	ErrBusy          = errors.New("link resolver busy")
)

type Asset struct {
	URL             string    `json:"url"`
	SourceURL       string    `json:"sourceURL,omitempty"`
	Kind            string    `json:"kind"` // image, video, audio
	ContentType     string    `json:"contentType,omitempty"`
	Width           int       `json:"width,omitempty"`
	Height          int       `json:"height,omitempty"`
	Duration        float64   `json:"duration,omitempty"`
	Size            int64     `json:"size,omitempty"`
	ObjectKey       string    `json:"objectKey,omitempty"` // nonempty only after an explicit CDN copy
	SHA256          string    `json:"sha256,omitempty"`
	ExpiresAt       time.Time `json:"expiresAt,omitzero"` // absolute CDN object deadline
	SourceExpiresAt time.Time `json:"sourceExpiresAt,omitzero"`
	// Extraction headers are backend-only. Cookie/authorization values must never
	// be included in a client result or sent to a different media origin.
	Headers http.Header `json:"-"`
	// Set only by isolated anonymous extraction after cookie name/domain checks.
	// It is never serialized and grants headers only to this exact origin.
	anonymousCookieOrigin string
	// Same-photo Instagram renditions are transient extraction data. Retries
	// retain the original aspect ratio and share the existing download budget.
	imageRenditions []string
	downloadBytes   int64
}

type Result struct {
	CacheUntil  time.Time `json:"-"`
	URL         string    `json:"url"`
	Provider    string    `json:"provider"`
	ID          string    `json:"id,omitempty"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Author      string    `json:"author,omitempty"`
	Assets      []Asset   `json:"assets,omitempty"`
	// Embed is an inert compatibility field for retained legacy fetchers.
	// The native resolver never reads or writes Iframely responses.
	Embed   *iframely.Response `json:"embed,omitempty"`
	Preview *Preview           `json:"preview,omitempty"`
}

// Preview contains public metadata. Only isolated widget rendering may use HTML.
// Image and file URLs are validated CDN deliveries, never third-party fallbacks.
type Preview struct {
	Kind        string    `json:"kind"` // card, widget, file
	Site        string    `json:"site,omitempty"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Image       string    `json:"image,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	HTML        string    `json:"html,omitempty"`
	ImageWidth  int       `json:"imageWidth,omitempty"`
	ImageHeight int       `json:"imageHeight,omitempty"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt,omitzero"`
	Reason      string    `json:"reason,omitempty"`
	File        *FileInfo `json:"file,omitempty"`
	ImageAsset  *Asset    `json:"imageAsset,omitempty"` // durable object ownership
}

type FileInfo struct {
	Asset  Asset  `json:"asset"`
	Name   string `json:"name"`
	Reason string `json:"reason,omitempty"`
}

type Connector interface {
	Resolve(context.Context, string) (Result, error)
}

type HTTPError struct {
	Provider   string
	Status     int
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string { return fmt.Sprintf("%s returned HTTP %d", e.Provider, e.Status) }
