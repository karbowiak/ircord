package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/iframely"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
)

type LinkMediaInput struct {
	ID        string `path:"id" maxLength:"200"`
	MessageID string `path:"messageId" maxLength:"200"`
	Body      struct {
		URL    string `json:"url" minLength:"1" maxLength:"8192"`
		Widget bool   `json:"widget,omitempty" doc:"Use the Iframely widget if a native source failed in the browser"`
	}
}

type LinkMediaAsset struct {
	URL         string     `json:"url"`
	Kind        string     `json:"kind" enum:"image,video,audio"`
	ContentType string     `json:"contentType,omitempty"`
	Width       int        `json:"width,omitempty"`
	Height      int        `json:"height,omitempty"`
	Duration    float64    `json:"duration,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty" doc:"Absolute expiry of this CDN copy; resolve the original message link again to refresh it"`
}
type LinkMediaWidget struct {
	URL         string  `json:"url"`
	External    bool    `json:"external" doc:"An HTTPS provider iframe; all other widget URLs are opaque-origin documents"`
	Token       string  `json:"token" doc:"Validates size messages from this widget document only"`
	Height      int     `json:"height"`
	AspectRatio float64 `json:"aspectRatio,omitempty" doc:"Responsive width/height ratio from the provider's wrapper"`
	ClickToLoad bool    `json:"clickToLoad,omitempty" doc:"Load a heavy news player only after the user activates it"`
	Thumbnail   string  `json:"thumbnail,omitempty" doc:"Iframely's preview image for a click-to-load player"`
}

// LinkMediaCard is a page summary the client renders itself, so its size follows from these
// fields before anything loads (an iframe would only know its height after measuring itself).
type LinkMediaCard struct {
	Site        string `json:"site"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty" doc:"HTTPS preview image"`
	ImageWidth  int    `json:"imageWidth,omitempty"`
	ImageHeight int    `json:"imageHeight,omitempty"`
	Icon        string `json:"icon,omitempty" doc:"HTTPS site icon"`
}
type LinkMediaResponse struct {
	URL         string           `json:"url"`
	Provider    string           `json:"provider"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Author      string           `json:"author,omitempty"`
	Assets      []LinkMediaAsset `json:"assets" nullable:"false"`
	Widget      *LinkMediaWidget `json:"widget,omitempty"`
	Card        *LinkMediaCard   `json:"card,omitempty"`
}

// Only the bounded, short-lived rendering cache keeps embed HTML. Nothing is
// written to message history, events, a gallery index, or shared browser caches.
type linkMediaEntry struct {
	owner, conversation, message, url, token, html string
	done                                           chan struct{}
	response                                       LinkMediaResponse
	err                                            error
	expires                                        time.Time
	bytes                                          int
	cacheUntil                                     time.Time
	storedMedia                                    bool
}
type linkMediaCache struct {
	mu      sync.Mutex
	entries map[string]*linkMediaEntry
	bytes   int
}

var errMediaBusy = errors.New("media resolver busy")

func (c *linkMediaCache) get(ctx context.Context, key string, entry *linkMediaEntry, load func(*linkMediaEntry) error) (LinkMediaResponse, error) {
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]*linkMediaEntry{}
	}
	now := time.Now()
	pending, owned := 0, 0
	var oldestKey string
	var oldest time.Time
	for k, e := range c.entries {
		if e.expires.IsZero() {
			pending++
			if e.owner == entry.owner {
				owned++
			}
			continue
		}
		if now.After(e.expires) {
			delete(c.entries, k)
			c.bytes -= e.bytes
			continue
		}
		if oldest.IsZero() || e.expires.Before(oldest) {
			oldest, oldestKey = e.expires, k
		}
	}
	if found := c.entries[key]; found != nil {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return LinkMediaResponse{}, ctx.Err()
		case <-found.done:
			return found.response, found.err
		}
	}
	if pending >= 4 || owned >= 2 {
		c.mu.Unlock()
		return LinkMediaResponse{}, errMediaBusy
	}
	if len(c.entries) >= 128 || c.bytes >= 16<<20 {
		if oldestKey == "" {
			c.mu.Unlock()
			return LinkMediaResponse{}, errMediaBusy
		}
		c.bytes -= c.entries[oldestKey].bytes
		delete(c.entries, oldestKey)
	}
	entry.done = make(chan struct{})
	c.entries[key] = entry
	c.mu.Unlock()
	err := load(entry)
	encoded, _ := json.Marshal(entry.response)
	c.mu.Lock()
	entry.err = err
	entry.expires = time.Now().Add(linkmedia.ResolutionCacheTTL)
	if !entry.cacheUntil.IsZero() && entry.cacheUntil.Before(entry.expires) {
		entry.expires = entry.cacheUntil
	}
	if entry.storedMedia {
		if recheck := time.Now().Add(linkmedia.StoredResultRecheck); recheck.Before(entry.expires) {
			entry.expires = recheck
		}
	}
	// A nearly expired source is deliberately not retained by the URL cache,
	// but the freshly minted document token still needs time to be opened.
	if err == nil && entry.response.Widget != nil && !entry.expires.After(time.Now()) {
		entry.expires = time.Now().Add(30 * time.Second)
	}
	if err != nil {
		entry.expires = time.Now().Add(15 * time.Second)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		entry.expires = time.Now()
	}
	entry.bytes = len(entry.html) + len(encoded)
	if entry.bytes > 1<<20 {
		entry.err = linkmedia.ErrLimit
		entry.response = LinkMediaResponse{}
		entry.html = ""
		entry.bytes = 0
		entry.expires = time.Now().Add(15 * time.Second)
	}
	c.bytes += entry.bytes
	close(entry.done)
	c.mu.Unlock()
	return entry.response, entry.err
}

func (s *Server) readableLink(account, conversation, message, raw string) error {
	msg, err := s.Manager.Message(account, conversation, message)
	if err != nil {
		return err
	}
	if !slices.Contains(linkmedia.URLs(msg.Text), raw) {
		return irc.ErrNotFound
	}
	return nil
}

func (s *Server) registerLinkMedia(router chi.Router) {
	huma.Register(s.API, huma.Operation{OperationID: "resolve-message-link", Method: http.MethodPost, Path: "/api/v1/conversations/{id}/messages/{messageId}/link-media", Summary: "Resolve a visible message link", Description: "Direct media files and supported social providers use optional CDN copies for up to 90 days, with original source delivery when caching is unavailable. Expired or missing copies refresh on demand. Other links and extraction failures use isolated Iframely widgets. Requires the retained source message in your current workspace and does not modify history."}, func(ctx context.Context, in *LinkMediaInput) (*Output[LinkMediaResponse], error) {
		account := owner(ctx)
		if err := s.readableLink(account, in.ID, in.MessageID, in.Body.URL); err != nil {
			return result(LinkMediaResponse{}, err)
		}
		if s.options.Media == nil {
			return nil, huma.Error503ServiceUnavailable("Link rendering is unavailable.")
		}
		hash := sha256.Sum256([]byte(account + "\x00" + in.ID + "\x00" + in.MessageID + "\x00" + in.Body.URL + map[bool]string{true: "widget", false: "source"}[in.Body.Widget]))
		value, err := s.linkMedia.get(ctx, hex.EncodeToString(hash[:]), &linkMediaEntry{owner: account, conversation: in.ID, message: in.MessageID, url: in.Body.URL}, func(entry *linkMediaEntry) error {
			var resolved linkmedia.Result
			var err error
			if in.Body.Widget {
				resolved, err = s.options.Media.ResolveWidget(ctx, in.Body.URL)
			} else {
				resolved, err = s.options.Media.ResolveSource(ctx, in.Body.URL)
			}
			if err != nil {
				return err
			}
			entry.cacheUntil = resolved.CacheUntil
			out := LinkMediaResponse{URL: resolved.URL, Provider: resolved.Provider, Title: resolved.Title, Description: resolved.Description, Author: resolved.Author, Assets: []LinkMediaAsset{}}
			for _, asset := range resolved.Assets {
				item := LinkMediaAsset{URL: asset.URL, Kind: asset.Kind, ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height, Duration: asset.Duration}
				if asset.ObjectKey != "" && !asset.ExpiresAt.IsZero() {
					item.ExpiresAt = &asset.ExpiresAt
					entry.storedMedia = true
				}
				out.Assets = append(out.Assets, item)
			}
			if resolved.Embed != nil && resolved.Embed.HTML == "" {
				// A page without a provider player: the client draws the summary itself.
				out.Card = summaryCard(*resolved.Embed, in.Body.URL)
				if out.Card == nil {
					return linkmedia.ErrUnavailable
				}
			} else if resolved.Embed != nil {
				markup := resolved.Embed.HTML
				if markup == "" {
					return linkmedia.ErrUnavailable
				}
				var nonce [24]byte
				if _, err := rand.Read(nonce[:]); err != nil {
					return err
				}
				entry.token = hex.EncodeToString(nonce[:])
				entry.html = markup
				frame, height, ratio := providerWidgetLayout(entry.html, in.Body.URL, s.options.PublicURL)
				if frame != "" && ratio == 0 {
					for _, links := range resolved.Embed.Links {
						for _, link := range links {
							if link.Href == frame {
								var supplied float64
								if json.Unmarshal(link.Media["aspect-ratio"], &supplied) == nil && supplied >= 0.05 && supplied <= 20 {
									ratio = supplied
								}
							}
						}
					}
				}
				out.Widget = &LinkMediaWidget{URL: "/api/v1/link-media/widgets/" + entry.token, Token: entry.token, Height: height}
				if frame != "" {
					out.Widget.URL = frame
					out.Widget.External = true
					out.Widget.AspectRatio = ratio
					applyWidgetOptions(out.Widget, *resolved.Embed)
				}
			}
			entry.response = out
			return nil
		})
		if errors.Is(err, errMediaBusy) || errors.Is(err, linkmedia.ErrBusy) {
			return nil, huma.Error429TooManyRequests("Link rendering is busy. Try again shortly.")
		}
		if err != nil {
			if errors.Is(err, linkmedia.ErrAgeRestricted) {
				return nil, huma.Error422UnprocessableEntity("This post is age-restricted. Open the original to view it on the site.")
			}
			if errors.Is(err, linkmedia.ErrRestricted) {
				return nil, huma.Error422UnprocessableEntity("This post isn’t publicly accessible. It may require signing in or age verification on the original site.")
			}
			var upstream *iframely.APIError
			if errors.As(err, &upstream) {
				switch {
				case upstream.Status == 403 || upstream.Code == "403":
					return nil, huma.Error422UnprocessableEntity("This site refused the preview request. Open the original to view it.")
				case upstream.Status == 404 || upstream.Code == "404":
					return nil, huma.Error422UnprocessableEntity("This post is unavailable or can’t be embedded. Open the original to check access.")
				}
			}
			return nil, huma.Error503ServiceUnavailable("This link could not be displayed. You can still open the original.")
		}
		// Authorization is checked again after potentially slow provider requests.
		if err := s.readableLink(account, in.ID, in.MessageID, in.Body.URL); err != nil {
			return result(LinkMediaResponse{}, err)
		}
		return result(value, nil)
	})
	router.Get("/api/v1/link-media/widgets/{token}", s.linkMediaWidget)
}

func (s *Server) linkMediaWidget(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	s.linkMedia.mu.Lock()
	var entry *linkMediaEntry
	for _, e := range s.linkMedia.entries {
		if e.owner == owner(r.Context()) && e.token == token && !e.expires.IsZero() && time.Now().Before(e.expires) && e.err == nil {
			copy := *e
			entry = &copy
			break
		}
	}
	s.linkMedia.mu.Unlock()
	if entry == nil || s.readableLink(entry.owner, entry.conversation, entry.message, entry.url) != nil {
		writeProblem(w, "widget unavailable", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	// Players such as YouTube require client identification. Only this origin
	// reaches cross-origin providers; message paths and widget tokens stay private.
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Content-Security-Policy", widgetCSP(s.options.PublicURL))
	_, _ = w.Write([]byte(widgetDocument(entry.html, entry.url, entry.token, r.URL.Query().Get("theme"))))
}
