package emotes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Endpoints locate the provider APIs and the CDNs their images may come from.
type Endpoints struct {
	SevenTVAPI string // https://7tv.io/v3
	SevenTVCDN string // https://cdn.7tv.app/emote/
	FFZAPI     string // https://api.frankerfacez.com/v1
	FFZCDN     string // https://cdn.frankerfacez.com/emote/
}

func (e Endpoints) withDefaults() Endpoints {
	if e.SevenTVAPI == "" {
		e.SevenTVAPI = "https://7tv.io/v3"
	}
	if e.SevenTVCDN == "" {
		e.SevenTVCDN = "https://cdn.7tv.app/emote/"
	}
	if e.FFZAPI == "" {
		e.FFZAPI = "https://api.frankerfacez.com/v1"
	}
	if e.FFZCDN == "" {
		e.FFZCDN = "https://cdn.frankerfacez.com/emote/"
	}
	return e
}

// ProviderEmote is the metadata a provider publishes for one emote.
type ProviderEmote struct {
	Provider  string
	ID        string
	Name      string
	Creator   string
	SourceURL string
	ImageURL  string
	StillURL  string // empty for still emotes
}

var (
	sevenTVID = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	ffzID     = regexp.MustCompile(`^[0-9]{1,10}$`)
)

type lookup struct {
	done    chan struct{}
	value   ProviderEmote
	err     error
	expires time.Time
}
type lookups struct {
	mu    sync.Mutex
	items map[string]*lookup
}

// Lookup returns a provider's metadata. Concurrent and repeated lookups of one emote share
// a single request; results are cached for an hour and failures for a minute.
func (s *Service) Lookup(ctx context.Context, provider, id string) (ProviderEmote, error) {
	switch provider {
	case "7tv":
		if !sevenTVID.MatchString(id) {
			return ProviderEmote{}, fail(ErrInvalid, "7TV emote IDs have 26 characters.")
		}
	case "ffz":
		if !ffzID.MatchString(id) {
			return ProviderEmote{}, fail(ErrInvalid, "FrankerFaceZ emote IDs are numbers.")
		}
	default:
		return ProviderEmote{}, fail(ErrInvalid, "Supported providers are 7tv and ffz.")
	}
	key := provider + ":" + id
	s.lookups.mu.Lock()
	if s.lookups.items == nil {
		s.lookups.items = map[string]*lookup{}
	}
	now := time.Now()
	for k, l := range s.lookups.items {
		if !l.expires.IsZero() && now.After(l.expires) {
			delete(s.lookups.items, k)
		}
	}
	l, ok := s.lookups.items[key]
	if !ok {
		l = &lookup{done: make(chan struct{})}
		s.lookups.items[key] = l
		go func() {
			fetch, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			value, err := s.fetchMeta(fetch, provider, id)
			s.lookups.mu.Lock()
			l.value, l.err = value, err
			l.expires = time.Now().Add(time.Hour)
			if err != nil {
				l.expires = time.Now().Add(time.Minute)
			}
			s.lookups.mu.Unlock()
			close(l.done)
		}()
	}
	s.lookups.mu.Unlock()
	select {
	case <-l.done:
		return l.value, l.err
	case <-ctx.Done():
		return ProviderEmote{}, ctx.Err()
	}
}

func (s *Service) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "IRCord emote catalog")
	res, err := s.options.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if res.StatusCode != 200 {
		return nil, errors.New("provider returned " + strconv.Itoa(res.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("provider response too large")
	}
	return data, nil
}

func (s *Service) fetchMeta(ctx context.Context, provider, id string) (ProviderEmote, error) {
	e := s.options.Endpoints
	api := e.SevenTVAPI + "/emotes/" + id
	if provider == "ffz" {
		api = e.FFZAPI + "/emote/" + id
	}
	raw, err := s.get(ctx, api, 256<<10)
	if errors.Is(err, ErrNotFound) {
		return ProviderEmote{}, fail(ErrNotFound, "The provider has no emote with that ID.")
	}
	if err != nil {
		return ProviderEmote{}, fail(ErrProvider, "The emote provider is unavailable. Try again later.")
	}
	var out ProviderEmote
	if provider == "7tv" {
		out, err = parseSevenTV(raw, id, e)
	} else {
		out, err = parseFFZ(raw, id, e)
	}
	if err != nil {
		return ProviderEmote{}, fail(ErrProvider, "The provider returned unusable emote data.")
	}
	return out, nil
}

func validName(name string) bool { return aliasPattern.MatchString(name) }
func short(value string) string {
	if len(value) > 64 {
		return value[:64]
	}
	return value
}

func parseSevenTV(raw []byte, id string, e Endpoints) (ProviderEmote, error) {
	var v struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Animated bool   `json:"animated"`
		Owner    *struct {
			DisplayName string `json:"display_name"`
		} `json:"owner"`
		Host struct {
			URL   string `json:"url"`
			Files []struct {
				Name       string `json:"name"`
				StaticName string `json:"static_name"`
				Format     string `json:"format"`
				Size       int64  `json:"size"`
			} `json:"files"`
		} `json:"host"`
	}
	if json.Unmarshal(raw, &v) != nil || v.ID != id || !validName(v.Name) {
		return ProviderEmote{}, errors.New("invalid 7TV emote")
	}
	base := v.Host.URL
	if strings.HasPrefix(base, "//") {
		base = "https:" + base
	}
	if base != e.SevenTVCDN+id {
		return ProviderEmote{}, errors.New("unexpected 7TV image host")
	}
	out := ProviderEmote{Provider: "7tv", ID: id, Name: v.Name, SourceURL: "https://7tv.app/emotes/" + id}
	if v.Owner != nil {
		out.Creator = short(v.Owner.DisplayName)
	}
	for _, want := range []string{"2x.webp", "1x.webp"} {
		for _, f := range v.Host.Files {
			if f.Name != want {
				continue
			}
			out.ImageURL = base + "/" + f.Name
			if v.Animated {
				if f.StaticName == "" || strings.ContainsAny(f.StaticName, "/?#") {
					return ProviderEmote{}, errors.New("missing still frame")
				}
				out.StillURL = base + "/" + f.StaticName
			}
			return out, nil
		}
	}
	return ProviderEmote{}, errors.New("no WebP image")
}

func parseFFZ(raw []byte, id string, e Endpoints) (ProviderEmote, error) {
	var v struct {
		Emote struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Owner *struct {
				DisplayName string `json:"display_name"`
				Name        string `json:"name"`
			} `json:"owner"`
			URLs     map[string]string `json:"urls"`
			Animated map[string]string `json:"animated"`
		} `json:"emote"`
	}
	if json.Unmarshal(raw, &v) != nil || strconv.FormatInt(v.Emote.ID, 10) != id || !validName(v.Emote.Name) {
		return ProviderEmote{}, errors.New("invalid FFZ emote")
	}
	prefix := e.FFZCDN + id + "/"
	pick := func(urls map[string]string) string {
		for _, scale := range []string{"2", "1"} {
			if u := urls[scale]; strings.HasPrefix(u, prefix) && !strings.ContainsAny(u[len(prefix):], "?#") && !strings.Contains(u, "..") {
				return u
			}
		}
		return ""
	}
	out := ProviderEmote{Provider: "ffz", ID: id, Name: v.Emote.Name, SourceURL: "https://www.frankerfacez.com/emoticon/" + id}
	if v.Emote.Owner != nil {
		out.Creator = short(v.Emote.Owner.DisplayName)
		if out.Creator == "" {
			out.Creator = short(v.Emote.Owner.Name)
		}
	}
	still := pick(v.Emote.URLs)
	if still == "" {
		return ProviderEmote{}, errors.New("no FFZ image")
	}
	out.ImageURL = still
	if animated := pick(v.Emote.Animated); animated != "" {
		out.ImageURL, out.StillURL = animated, still
	}
	return out, nil
}

// Import copies a provider emote into the catalog. The shortcode defaults to the provider's
// name in lowercase; the provider's own name is kept as the canonical name and an alias.
func (s *Service) Import(ctx context.Context, provider, id string, in New) (Emote, error) {
	meta, err := s.Lookup(ctx, provider, strings.TrimSpace(id))
	if err != nil {
		return Emote{}, err
	}
	download := func(url string) (Image, error) {
		data, err := s.get(ctx, url, s.options.MaxBytes)
		if err != nil {
			return Image{}, fail(ErrProvider, "The emote image could not be downloaded.")
		}
		return s.validate(data)
	}
	img, err := download(meta.ImageURL)
	if err != nil {
		return Emote{}, err
	}
	var still Image
	if meta.StillURL != "" {
		if still, err = download(meta.StillURL); err != nil {
			return Emote{}, err
		}
		if still.Animated {
			return Emote{}, fail(ErrProvider, "The provider's still frame is animated.")
		}
	} else if img.Animated {
		if still, err = gifStill(img); err != nil {
			return Emote{}, fail(ErrProvider, "The provider has no still frame for this animated emote.")
		}
	}
	if in.Name == "" {
		in.Name = meta.Name
	}
	return s.add(ctx, in, Emote{Source: provider, ProviderID: meta.ID, SourceURL: meta.SourceURL, Creator: meta.Creator}, img, still)
}

// Seed is a provider emote added once to a new catalog.
type Seed struct {
	Provider, ID, Shortcode string
}

// DefaultSeeds are pinned provider IDs verified for issue #45. Names have many uploads, so
// a seed never searches by name.
var DefaultSeeds = []Seed{
	{"7tv", "01GA29CZ2R000C36HNE7Z0DQXD", "kekw"},
	{"7tv", "01FBFNMS380002BEVDT2R9QCDR", "kekl"},
	{"7tv", "01FKF8QHVR000B1FXDJGM9EDWQ", "kekleo"},
	{"7tv", "01HGXBVKPR00025ZBQF6KKY8YD", "oooo"},
	{"ffz", "481082", "notlikethis"},
}

// SeedOnce imports each seed that was never applied. A seed counts as applied once imported
// or once its name or provider entry already exists, so removing a seeded emote is final.
// It returns how many seeds still need a retry because a provider failed.
func (s *Service) SeedOnce(ctx context.Context, seeds []Seed) int {
	pending := 0
	for _, seed := range seeds {
		key := "seed:" + seed.Provider + ":" + seed.ID
		var value string
		if s.options.Store.DB.QueryRowContext(ctx, "SELECT value FROM emote_state WHERE key=?", key).Scan(&value) == nil {
			continue
		}
		_, err := s.Import(ctx, seed.Provider, seed.ID, New{Shortcode: seed.Shortcode, AddedBy: "system"})
		if err != nil && !errors.Is(err, ErrConflict) {
			slog.Warn("Emote seed will retry", "provider", seed.Provider, "id", seed.ID, "error", err)
			pending++
			continue
		}
		_, _ = s.options.Store.DB.ExecContext(ctx, "INSERT OR IGNORE INTO emote_state(key,value) VALUES(?,?)", key, time.Now().UTC().Format(time.RFC3339))
	}
	return pending
}

// SeedInBackground applies seeds now and retries provider failures with backoff until
// every seed is applied or ctx ends.
func (s *Service) SeedInBackground(ctx context.Context, seeds []Seed) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		delay := time.Minute
		for s.SeedOnce(ctx, seeds) > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			delay = min(delay*2, 6*time.Hour)
		}
	}()
	return done
}
