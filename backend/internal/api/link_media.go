package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
)

type LinkMediaInput struct {
	ID        string `path:"id" maxLength:"200"`
	MessageID string `path:"messageId" maxLength:"200"`
	Body      struct {
		URL    string `json:"url" minLength:"1" maxLength:"8192"`
		Widget bool   `json:"widget,omitempty" deprecated:"true" doc:"Ignored; all links use the same resolver"`
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
	URL         string     `json:"url"`
	External    bool       `json:"external" doc:"An HTTPS provider iframe; all other widget URLs are opaque-origin documents"`
	Token       string     `json:"token" doc:"Validates size messages from this widget document only"`
	Height      int        `json:"height"`
	AspectRatio float64    `json:"aspectRatio,omitempty" doc:"Validated widget width divided by height"`
	ClickToLoad bool       `json:"clickToLoad,omitempty" doc:"Load the provider player after the user activates it"`
	Thumbnail   string     `json:"thumbnail,omitempty" doc:"Validated CDN preview image"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty" doc:"Refresh before this widget document or its CDN thumbnail expires"`
}

// LinkMediaCard is a page summary the client renders itself, so its size follows from these
// fields before anything loads (an iframe would only know its height after measuring itself).
type LinkMediaCard struct {
	Site        string     `json:"site"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Image       string     `json:"image,omitempty" doc:"Validated CDN preview image"`
	ImageWidth  int        `json:"imageWidth,omitempty"`
	ImageHeight int        `json:"imageHeight,omitempty"`
	Icon        string     `json:"icon,omitempty" doc:"Validated CDN site icon"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	Reason      string     `json:"reason,omitempty" enum:"unsupported_type,type_mismatch,too_large,invalid_file,fetch_failed,storage_unavailable,no_oembed,restricted"`
}

type LinkMediaFile struct {
	Name        string     `json:"name"`
	URL         string     `json:"url" doc:"Validated CDN copy or authorized IRCord file; empty for rejected or uncached files"`
	ContentType string     `json:"contentType" doc:"Detected content type when available"`
	Size        int64      `json:"size" doc:"Observed file size when available"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	Reason      string     `json:"reason,omitempty" enum:"unsupported_type,type_mismatch,too_large,invalid_file,fetch_failed,storage_unavailable,no_oembed,restricted"`
}
type LinkMediaResponse struct {
	URL         string           `json:"url"`
	Kind        string           `json:"kind" enum:"media,file,card,widget"`
	Provider    string           `json:"provider"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Author      string           `json:"author,omitempty"`
	Assets      []LinkMediaAsset `json:"assets" nullable:"false"`
	Widget      *LinkMediaWidget `json:"widget,omitempty"`
	Card        *LinkMediaCard   `json:"card,omitempty"`
	File        *LinkMediaFile   `json:"file,omitempty"`
}

// Prepared widget markup and tokens stay in this bounded, account-scoped cache.
// Raw public oEmbed metadata may persist in the durable resolver cache. Neither
// is written to message history or events; responses bypass shared browser caches.
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
	c.mu.Lock()
	entry.err = err
	entry.expires = time.Now().Add(linkmedia.ResolutionCacheTTL)
	if !entry.cacheUntil.IsZero() && entry.cacheUntil.Before(entry.expires) {
		entry.expires = entry.cacheUntil
	}
	// Delivery metadata rechecks must not shorten a widget snapshot's lifetime:
	// its metadata and CDN deadlines already bound its markup and document token.
	if entry.storedMedia && entry.response.Widget == nil {
		if recheck := time.Now().Add(linkmedia.StoredResultRecheck); recheck.Before(entry.expires) {
			entry.expires = recheck
		}
	}
	if err != nil {
		entry.expires = time.Now().Add(15 * time.Second)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		entry.expires = time.Now()
	}
	if widget := entry.response.Widget; widget != nil {
		if widget.ExpiresAt != nil && widget.ExpiresAt.Before(entry.expires) {
			entry.expires = *widget.ExpiresAt
		}
		widget.ExpiresAt = mediaExpiry(entry.expires)
	}
	encoded, _ := json.Marshal(entry.response)
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
	text := textOutsideLinkCode(msg.Text)
	// Local uploads can use a private configured app origin. This exception
	// authorizes only an exact retained capability, never a public HTTP fetch.
	if _, _, ok := s.ownedFileRoute(raw); ok {
		if retainedOwnedLink(text, raw) {
			return nil
		}
	} else if slices.Contains(linkmedia.URLs(text), raw) {
		return nil
	}
	return irc.ErrNotFound
}

func (s *Server) registerLinkMedia(router chi.Router) {
	huma.Register(s.API, huma.Operation{OperationID: "resolve-message-link", Method: http.MethodPost, Path: "/api/v1/conversations/{id}/messages/{messageId}/link-media", Summary: "Resolve a visible message link", Description: "Returns validated CDN media, authorized IRCord files, metadata cards, or isolated widgets through one resolver. Rejected or unavailable content returns an information card or file record with a reason. Requires the retained source message in your current workspace and does not modify history."}, func(ctx context.Context, in *LinkMediaInput) (*Output[LinkMediaResponse], error) {
		account := owner(ctx)
		if err := s.readableLink(account, in.ID, in.MessageID, in.Body.URL); err != nil {
			return result(LinkMediaResponse{}, err)
		}
		// Owned capabilities are never shared with the public resolver or CDN.
		// Download checks token validity and original storage on every request.
		if value, handled := s.ownedLinkMedia(ctx, in.Body.URL); handled {
			if err := s.readableLink(account, in.ID, in.MessageID, in.Body.URL); err != nil {
				return result(LinkMediaResponse{}, err)
			}
			return result(value, nil)
		}
		hash := sha256.Sum256([]byte(account + "\x00" + in.ID + "\x00" + in.MessageID + "\x00" + in.Body.URL))
		value, err := s.linkMedia.get(ctx, hex.EncodeToString(hash[:]), &linkMediaEntry{owner: account, conversation: in.ID, message: in.MessageID, url: in.Body.URL}, func(entry *linkMediaEntry) error {
			return s.resolveLinkMedia(ctx, entry)
		})
		// Authorization is checked again after potentially slow provider requests.
		if err := s.readableLink(account, in.ID, in.MessageID, in.Body.URL); err != nil {
			return result(LinkMediaResponse{}, err)
		}
		if err != nil {
			value = linkFallback(in.Body.URL, mediaFailureReason(err))
		}
		return result(value, nil)
	})
	router.Get("/api/v1/link-media/widgets/{token}", s.linkMediaWidget)
}

func linkFallback(raw, reason string) LinkMediaResponse {
	site := "Link"
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		site = strings.TrimPrefix(u.Hostname(), "www.")
	}
	return LinkMediaResponse{URL: raw, Kind: "card", Provider: "link", Assets: []LinkMediaAsset{}, Card: &LinkMediaCard{Site: site, Title: site, Reason: reason}}
}

func mediaFailureReason(err error) string {
	switch {
	case errors.Is(err, linkmedia.ErrRestricted), errors.Is(err, linkmedia.ErrAgeRestricted):
		return "restricted"
	case errors.Is(err, linkmedia.ErrLimit):
		return "too_large"
	case errors.Is(err, linkmedia.ErrUnsupported):
		return "unsupported_type"
	default:
		return "fetch_failed"
	}
}

func linkMediaReason(reason string) string {
	switch reason {
	case "", "unsupported_type", "type_mismatch", "too_large", "invalid_file", "fetch_failed", "storage_unavailable", "no_oembed", "restricted":
		return reason
	default:
		return "fetch_failed"
	}
}

func mediaExpiry(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func (s *Server) resolveLinkMedia(ctx context.Context, entry *linkMediaEntry) error {
	entry.response = linkFallback(entry.url, "storage_unavailable")
	if s.options.Media == nil {
		entry.cacheUntil = time.Now().Add(15 * time.Second)
		return nil
	}
	resolved, err := s.options.Media.Resolve(ctx, entry.url)
	if err != nil {
		entry.response.Card.Reason = mediaFailureReason(err)
		entry.cacheUntil = time.Now().Add(15 * time.Second)
		return nil
	}
	entry.cacheUntil = resolved.CacheUntil
	out := linkFallback(entry.url, "no_oembed")
	if resolved.Provider != "" {
		out.Provider = resolved.Provider
	}
	out.Title, out.Description, out.Author = resolved.Title, resolved.Description, resolved.Author
	for _, asset := range resolved.Assets {
		if nativeMediaKind(asset.ContentType) != asset.Kind || !s.options.Media.IsDeliveryURL(asset.URL) ||
			!asset.ExpiresAt.IsZero() && !asset.ExpiresAt.After(time.Now()) {
			continue
		}
		if !slices.Contains([]string{"image", "video", "audio"}, asset.Kind) {
			continue
		}
		out.Assets = append(out.Assets, LinkMediaAsset{URL: asset.URL, Kind: asset.Kind, ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height, Duration: asset.Duration, ExpiresAt: mediaExpiry(asset.ExpiresAt)})
		entry.storedMedia = true
	}
	if len(out.Assets) > 0 {
		// Unknown page URLs have no native layout contract before discovery.
		// Keep their oEmbed photos inside the fixed metadata-card renderer.
		if resolved.Provider != "reddit" && !linkmedia.DirectFileCandidate(entry.url) {
			first := out.Assets[0]
			out.Assets = out.Assets[:0]
			out.Card.Reason = ""
			if out.Title != "" {
				out.Card.Title = out.Title
			}
			out.Card.Description = out.Description
			if first.Kind == "image" {
				out.Card.Image, out.Card.ImageWidth, out.Card.ImageHeight = first.URL, first.Width, first.Height
				out.Card.ExpiresAt = first.ExpiresAt
			}
			entry.response = out
			return nil
		}
		out.Kind, out.Card = "media", nil
		entry.response = out
		return nil
	}
	preview := resolved.Preview
	if preview == nil {
		// Legacy Embed data is deliberately ignored on this route.
		entry.response = out
		return nil
	}
	card := out.Card
	card.Reason = linkMediaReason(preview.Reason)
	if preview.Site != "" {
		card.Site = preview.Site
	}
	if preview.Title != "" {
		card.Title = preview.Title
	} else if resolved.Title != "" {
		card.Title = resolved.Title
	}
	card.Description = preview.Description
	if card.Description == "" {
		card.Description = resolved.Description
	}
	card.ExpiresAt = mediaExpiry(preview.ExpiresAt)
	if !preview.ExpiresAt.IsZero() && (entry.cacheUntil.IsZero() || preview.ExpiresAt.Before(entry.cacheUntil)) {
		entry.cacheUntil = preview.ExpiresAt
	}
	if preview.ExpiresAt.IsZero() || preview.ExpiresAt.After(time.Now()) {
		if s.options.Media.IsDeliveryURL(preview.Image) {
			card.Image, card.ImageWidth, card.ImageHeight = preview.Image, max(0, preview.ImageWidth), max(0, preview.ImageHeight)
			entry.storedMedia = true
		}
		if s.options.Media.IsDeliveryURL(preview.Icon) {
			card.Icon = preview.Icon
			entry.storedMedia = true
		}
	}
	if preview.File != nil {
		file := preview.File
		out.File = &LinkMediaFile{Name: file.Name, ContentType: file.Asset.ContentType, Size: max(0, file.Asset.Size), Reason: linkMediaReason(file.Reason)}
		if out.File.Reason == "" {
			out.File.Reason = card.Reason
		}
		if out.File.Reason == "" && s.options.Media.IsDeliveryURL(file.Asset.URL) &&
			(file.Asset.ExpiresAt.IsZero() || file.Asset.ExpiresAt.After(time.Now())) {
			out.File.URL, out.File.ExpiresAt = file.Asset.URL, mediaExpiry(file.Asset.ExpiresAt)
			entry.storedMedia = true
		} else if out.File.Reason == "" {
			out.File.Reason = "storage_unavailable"
		}
		out.Kind, out.Card = "file", nil
	} else if preview.Kind == "widget" && preview.HTML != "" {
		markup, frame, height, ratio, safe := prepareGenericWidget(preview.HTML, entry.url, s.options.PublicURL, s.options.Media.IsDeliveryURL)
		// Only recognized player frames have a controlled UI adapter. Static
		// or otherwise unknown rich markup remains metadata in a fixed card.
		if safe && frame != "" {
			var nonce [24]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				card.Reason = "fetch_failed"
			} else {
				entry.token, entry.html = hex.EncodeToString(nonce[:]), markup
				height, ratio = standaloneWidgetDimensions(frame)
				out.Widget = &LinkMediaWidget{URL: frame, External: true, Token: entry.token, Height: height, AspectRatio: ratio, Thumbnail: card.Image, ExpiresAt: card.ExpiresAt}
				out.Kind, out.Card = "widget", nil
			}
		} else if card.Reason == "" {
			card.Reason = "unsupported_type"
		}
	}
	entry.response = out
	return nil
}

// Native assets are restricted to formats the file service or public resolver
// validates. Documents and unrecognized types use the separate file record.
func nativeMediaKind(contentType string) string {
	contentType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif", "image/bmp":
		return "image"
	case "video/mp4", "video/webm", "video/quicktime", "video/ogg":
		return "video"
	case "audio/mpeg", "audio/mp3", "audio/mp4", "audio/x-m4a", "audio/ogg", "audio/wav", "audio/x-wav", "audio/flac", "audio/x-flac", "audio/aac", "audio/webm":
		return "audio"
	default:
		return ""
	}
}

func (s *Server) ownedFileRoute(raw string) (id, token string, ok bool) {
	u, err := url.Parse(raw)
	app, appErr := url.Parse(s.options.PublicURL)
	if err != nil || appErr != nil || app.Host == "" || app.User != nil || app.RawQuery != "" || app.Fragment != "" ||
		(app.Scheme != "http" && app.Scheme != "https") || u.Scheme != app.Scheme || u.Host != app.Host || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return "", "", false
	}
	prefix := strings.TrimRight(app.EscapedPath(), "/") + "/files/"
	if !strings.HasPrefix(u.EscapedPath(), prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(u.EscapedPath(), prefix), "/")
	if len(parts) != 3 || len(parts[0]) != 32 || len(parts[1]) != 48 || parts[2] == "" ||
		strings.Trim(parts[0], "0123456789abcdef") != "" || strings.Trim(parts[1], "0123456789abcdef") != "" { // gitleaks:allow -- hexadecimal validation alphabet
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (s *Server) ownedLinkMedia(ctx context.Context, raw string) (LinkMediaResponse, bool) {
	u, err := url.Parse(raw)
	if err != nil || !strings.Contains(u.Path, "/files/") {
		return LinkMediaResponse{}, false
	}
	out := linkFallback(raw, "restricted")
	out.Kind, out.Card = "file", nil
	out.File = &LinkMediaFile{Name: path.Base(u.Path), Reason: "restricted"}
	id, token, ok := s.ownedFileRoute(raw)
	if !ok || s.options.Files == nil {
		return out, true
	}
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	record, body, err := s.options.Files.Download(check, id, token)
	if body != nil {
		_ = body.Close()
	}
	if err != nil || record.URL != raw || record.Size <= 0 {
		return out, true
	}
	out.Provider, out.Title = "ircord", record.Name
	out.File.Name, out.File.ContentType, out.File.Size = record.Name, record.ContentType, record.Size
	if kind := nativeMediaKind(record.ContentType); kind != "" {
		out.Kind, out.File = "media", nil
		out.Assets = append(out.Assets, LinkMediaAsset{URL: raw, Kind: kind, ContentType: record.ContentType, Width: record.Width, Height: record.Height})
		return out, true
	}
	contentType, _, _ := mime.ParseMediaType(record.ContentType)
	switch contentType {
	case "application/pdf", "text/plain", "application/zip", "application/octet-stream":
		out.File.URL, out.File.Reason = raw, ""
	default:
		out.File.Reason = "unsupported_type"
	}
	return out, true
}

var retainedOwnedURL = regexp.MustCompile(`https?://[^\s<>"'\x00-\x1f]+`)

func retainedOwnedLink(text, raw string) bool {
	for _, candidate := range retainedOwnedURL.FindAllString(text, -1) {
		candidate = strings.TrimRight(candidate, ".,!?;:")
		for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
			for strings.HasSuffix(candidate, pair[1]) && strings.Count(candidate, pair[1]) > strings.Count(candidate, pair[0]) {
				candidate = strings.TrimSuffix(candidate, pair[1])
			}
		}
		if candidate == raw {
			return true
		}
	}
	return false
}

// Match the message renderer's closed, line-start fences and IRC monospace
// controls. Links displayed as code cannot authorize a preview request.
func textOutsideLinkCode(text string) string {
	var plain strings.Builder
	pending, at := 0, 0
	for at < len(text) {
		next := strings.Index(text[at:], "```")
		if next < 0 {
			break
		}
		at += next
		if at > 0 && text[at-1] != '\n' {
			at += 3
			continue
		}
		close := strings.Index(text[at+3:], "```")
		if close < 0 {
			break
		}
		close += at + 3
		if strings.TrimSpace(text[at+3:close]) == "" {
			at += 3
			continue
		}
		plain.WriteString(text[pending:at])
		plain.WriteByte(' ')
		pending, at = close+3, close+3
	}
	plain.WriteString(text[pending:])
	var visible strings.Builder
	mono := false
	for _, char := range plain.String() {
		switch char {
		case '\x11':
			mono = !mono
			visible.WriteByte(' ')
		case '\x0f':
			mono = false
			visible.WriteByte(' ')
		default:
			if !mono {
				visible.WriteRune(char)
			}
		}
	}
	return visible.String()
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
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	// Players such as YouTube require client identification. Only this origin
	// reaches cross-origin providers; message paths and widget tokens stay private.
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Content-Security-Policy", widgetCSP(s.options.PublicURL, entry.token, entry.html))
	_, _ = w.Write([]byte(widgetDocument(entry.html, entry.url, entry.token, r.URL.Query().Get("theme"))))
}
