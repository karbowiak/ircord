package emotes

import (
	"context"
	"strings"
	"sync"
	"time"
)

type DiscoveryItem struct {
	Provider   string `json:"provider"`
	ProviderID string `json:"providerId"`
	Name       string `json:"name"`
	Creator    string `json:"creator"`
	SourceURL  string `json:"sourceUrl"`
	PreviewURL string `json:"previewUrl"`
	License    string `json:"license"`
	Animated   bool   `json:"animated"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Category   string `json:"category"`
	Installed  bool   `json:"installed"`
}
type DiscoveryCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type DiscoveryFilters struct {
	Sources    []string            `json:"sources"`
	Categories []DiscoveryCategory `json:"categories"`
	Animations []string            `json:"animations"`
}
type DiscoveryResult struct {
	Items      []DiscoveryItem  `json:"items"`
	Filters    DiscoveryFilters `json:"filters"`
	NextCursor string           `json:"nextCursor"`
	Stale      bool             `json:"stale"`
}
type DiscoveryQuery struct {
	Source, Query, Category, Animation, Cursor string
	Limit                                      int
}
type discoveryEntry struct {
	item DiscoveryItem
	meta ProviderEmote
}
type previewEntry struct {
	image   Image
	expires time.Time
}
type previewLookup struct {
	done    chan struct{}
	image   Image
	err     error
	waiters int
	cancel  context.CancelFunc
}
type discoveryCache struct {
	mu           sync.Mutex
	previews     map[string]previewEntry
	previewSlots chan struct{}
	pending      map[string]*previewLookup
}

func (s *Service) discoverEntry(ctx context.Context, id string) (discoveryEntry, error) {
	if !websiteSlug.MatchString(id) || len(id) > 100 {
		return discoveryEntry{}, fail(ErrInvalid, "Invalid emoji.gg ID.")
	}
	if entry, ok := s.websiteEntry(id); ok {
		return entry, nil
	}
	meta, err := s.websiteDetail(ctx, id)
	if err != nil {
		return discoveryEntry{}, err
	}
	return discoveryEntry{meta: meta}, nil
}

func (s *Service) Discover(ctx context.Context, q DiscoveryQuery) (DiscoveryResult, error) {
	if q.Source == "" {
		q.Source = "emoji.gg"
	}
	if q.Animation == "" {
		q.Animation = "all"
	}
	if q.Limit == 0 {
		q.Limit = 30
	}
	q.Query = strings.ToLower(strings.TrimSpace(q.Query))
	if q.Source != "emoji.gg" || q.Category != "" || (q.Animation != "all" && q.Animation != "static" && q.Animation != "animated") || q.Limit < 1 || q.Limit > 50 || len(q.Query) > 100 || len(q.Cursor) > 512 {
		return DiscoveryResult{}, fail(ErrInvalid, "Invalid discovery filter or page.")
	}
	return s.discoverWebsite(ctx, q)
}

// Preview shares requests by provider ID and bounds both the queue and active downloads.
func (s *Service) Preview(ctx context.Context, provider, id string) (Image, error) {
	if provider != "emoji.gg" {
		return Image{}, fail(ErrInvalid, "Unknown provider.")
	}
	entry, err := s.discoverEntry(ctx, id)
	if err != nil {
		return Image{}, err
	}
	s.discovery.mu.Lock()
	if s.discovery.previewSlots == nil {
		s.discovery.previewSlots = make(chan struct{}, 2)
		s.discovery.previews = map[string]previewEntry{}
		s.discovery.pending = map[string]*previewLookup{}
	}
	if p, ok := s.discovery.previews[id]; ok && time.Now().Before(p.expires) {
		s.discovery.mu.Unlock()
		return p.image, nil
	}
	l, ok := s.discovery.pending[id]
	if !ok {
		if len(s.discovery.pending) >= 64 {
			s.discovery.mu.Unlock()
			return Image{}, fail(ErrProvider, "Preview capacity is busy. Try again shortly.")
		}
		fetchCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		l = &previewLookup{done: make(chan struct{}), cancel: cancel}
		s.discovery.pending[id] = l
		slots := s.discovery.previewSlots
		go func() {
			defer cancel()
			var img Image
			var err error
			select {
			case slots <- struct{}{}:
				fetch, stop := context.WithTimeout(fetchCtx, 15*time.Second)
				var raw []byte
				raw, err = s.get(fetch, entry.meta.ImageURL, s.options.MaxBytes)
				stop()
				<-slots
				if err == nil {
					img, err = s.validate(raw)
				}
				if err == nil && (strings.HasSuffix(entry.meta.ImageURL, ".gif") && img.ContentType != "image/gif" || strings.HasSuffix(entry.meta.ImageURL, ".png") && img.ContentType != "image/png") {
					err = fail(ErrProvider, "The emoji preview has an unexpected image type.")
				}
			case <-fetchCtx.Done():
				err = fetchCtx.Err()
			}
			if err != nil {
				err = fail(ErrProvider, "The emoji preview is unavailable.")
			}
			s.discovery.mu.Lock()
			if err == nil {
				if len(s.discovery.previews) >= 32 {
					for key := range s.discovery.previews {
						delete(s.discovery.previews, key)
						break
					}
				}
				s.discovery.previews[id] = previewEntry{img, time.Now().Add(10 * time.Minute)}
			}
			l.image, l.err = img, err
			if s.discovery.pending[id] == l {
				delete(s.discovery.pending, id)
			}
			close(l.done)
			s.discovery.mu.Unlock()
		}()
	}
	l.waiters++
	s.discovery.mu.Unlock()
	defer func() {
		s.discovery.mu.Lock()
		l.waiters--
		if l.waiters == 0 && s.discovery.pending[id] == l {
			delete(s.discovery.pending, id)
			l.cancel()
		}
		s.discovery.mu.Unlock()
	}()
	select {
	case <-l.done:
		return l.image, l.err
	case <-ctx.Done():
		return Image{}, ctx.Err()
	}
}

func (s *Service) PreviewStatic(ctx context.Context, provider, id string) (Image, error) {
	img, err := s.Preview(ctx, provider, id)
	if err != nil || !img.Animated {
		return img, err
	}
	return gifStill(img)
}
