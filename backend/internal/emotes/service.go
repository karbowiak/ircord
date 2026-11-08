// Package emotes owns the instance-wide custom emote catalog: every account on one IRCord
// backend shares it. Images, whether uploaded or imported from a provider, are copied into
// IRCord's own blob storage, so rendering never contacts a provider and the catalog does not
// depend on the account that added an entry. Message text keeps the readable :shortcode:,
// which is all IRC carries.
package emotes

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ircord/backend/internal/filehost"
	"ircord/backend/internal/storage"
)

// Emote is one catalog entry. It carries no account data: the account that added it is kept
// privately for auditing only.
type Emote struct {
	ID         string    `json:"id" doc:"Stable instance emote ID. Clients namespace it as instance:<id>."`
	Name       string    `json:"name" doc:"Canonical name, such as the provider's original casing."`
	Shortcode  string    `json:"shortcode" doc:"Lowercase :shortcode: written into message text."`
	Aliases    []string  `json:"aliases" doc:"Further names that render and complete to this emote."`
	Retired    []string  `json:"retired" doc:"Former names. They still render in message history but are not offered for new messages, and a new emote may claim them."`
	Enabled    bool      `json:"enabled"`
	Animated   bool      `json:"animated"`
	URL        string    `json:"url"`
	StaticURL  string    `json:"staticUrl" doc:"Still frame, used when animation is off. Equals url for still images."`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Source     string    `json:"source" enum:"upload,7tv,ffz"`
	ProviderID string    `json:"providerId,omitempty"`
	SourceURL  string    `json:"sourceUrl,omitempty"`
	Creator    string    `json:"creator,omitempty" doc:"Attribution: the provider's creator, or one given for an upload."`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// EmoteCatalog is the versioned list every account reads.
type EmoteCatalog struct {
	Version   int64    `json:"version" doc:"Changes whenever the catalog changes; emotes.updated carries the new value."`
	CanManage bool     `json:"canManage" doc:"Whether this account may upload, import, edit and remove emotes."`
	MaxBytes  int64    `json:"maxBytes"`
	Types     []string `json:"types"`
	Providers []string `json:"providers"`
	Items     []Emote  `json:"items" doc:"Enabled entries; managers also receive disabled ones."`
}

// EmotePatch changes an entry; nil fields are kept.
type EmotePatch struct {
	Name      *string   `json:"name,omitempty" maxLength:"64"`
	Shortcode *string   `json:"shortcode,omitempty" maxLength:"34"`
	Aliases   *[]string `json:"aliases,omitempty" maxItems:"8"`
	Enabled   *bool     `json:"enabled,omitempty"`
	Creator   *string   `json:"creator,omitempty" maxLength:"64"`
}

// New describes an entry being added.
type New struct {
	Name      string
	Shortcode string
	Aliases   []string
	Creator   string
	AddedBy   string
}

type record struct {
	Emote
	Assets []string `json:"assets"`
}

var (
	ErrNotFound = errors.New("emote not found")
	ErrInvalid  = errors.New("invalid emote")
	ErrConflict = errors.New("emote name conflict")
	ErrProvider = errors.New("emote provider unavailable")
)

// Error carries a user-facing detail while matching one of the sentinel errors above.
type Error struct {
	Kind   error
	Detail string
}

func (e *Error) Error() string                  { return e.Detail }
func (e *Error) Unwrap() error                  { return e.Kind }
func fail(kind error, f string, a ...any) error { return &Error{kind, fmt.Sprintf(f, a...)} }

type Options struct {
	Store     *storage.Store
	Blob      filehost.Blob
	PublicURL string
	Client    *http.Client
	// MaxBytes bounds every stored image. Defaults to 1 MiB.
	MaxBytes int64
	// Endpoints override the provider API and CDN locations (tests use local fixtures).
	Endpoints Endpoints
}

type Service struct {
	options Options
	mu      sync.Mutex
	items   map[string]*record
	version int64
	subsMu  sync.Mutex
	subs    map[chan int64]struct{}
	lookups lookups
}

var (
	shortcodePattern = regexp.MustCompile(`^[a-z0-9_]{2,32}$`)
	aliasPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{2,32}$`)
)

const maxRetired = 8

func Open(options Options) (*Service, error) {
	if options.Store == nil || options.Blob == nil {
		return nil, fmt.Errorf("emote storage is required")
	}
	if options.MaxBytes <= 0 {
		options.MaxBytes = 1 << 20
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 20 * time.Second}
	}
	client := *options.Client
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	options.Client = &client
	options.Endpoints = options.Endpoints.withDefaults()
	options.PublicURL = strings.TrimRight(options.PublicURL, "/")
	_, err := options.Store.DB.Exec(`CREATE TABLE IF NOT EXISTS emotes(id TEXT PRIMARY KEY,added_by TEXT NOT NULL,created_ns INTEGER NOT NULL,payload BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS emote_assets(object_key TEXT PRIMARY KEY,emote_id TEXT NOT NULL,backend TEXT NOT NULL,mime TEXT NOT NULL,size INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS emote_state(key TEXT PRIMARY KEY,value TEXT NOT NULL);`)
	if err != nil {
		return nil, err
	}
	s := &Service{options: options, items: map[string]*record{}, subs: map[chan int64]struct{}{}}
	var version string
	if err = options.Store.DB.QueryRow("SELECT value FROM emote_state WHERE key='version'").Scan(&version); err == nil {
		s.version, _ = strconv.ParseInt(version, 10, 64)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	rows, err := options.Store.DB.Query("SELECT payload FROM emotes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var r record
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		s.items[r.ID] = &r
	}
	return s, rows.Err()
}

// Catalog returns enabled entries in shortcode order, or every entry for a manager.
func (s *Service) Catalog(manage bool) EmoteCatalog {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []Emote{}
	for _, r := range s.items {
		if r.Enabled || manage {
			items = append(items, r.copy())
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Shortcode < items[j].Shortcode })
	return EmoteCatalog{Version: s.version, CanManage: manage, MaxBytes: s.options.MaxBytes, Types: imageTypes, Providers: []string{"7tv", "ffz"}, Items: items}
}

func (s *Service) Version() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.version
}

func (r *record) copy() Emote {
	e := r.Emote
	e.Aliases = append([]string{}, r.Aliases...)
	e.Retired = append([]string{}, r.Retired...)
	return e
}

// Subscribe delivers the catalog version after each change. A slow reader only ever sees
// the newest version.
func (s *Service) Subscribe() (<-chan int64, func()) {
	ch := make(chan int64, 1)
	s.subsMu.Lock()
	s.subs[ch] = struct{}{}
	s.subsMu.Unlock()
	return ch, func() {
		s.subsMu.Lock()
		delete(s.subs, ch)
		s.subsMu.Unlock()
	}
}
func (s *Service) notify(version int64) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for ch := range s.subs {
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- version:
		default:
		}
	}
}

func clean(name string) string { return strings.Trim(strings.TrimSpace(name), ":") }

// names validates and normalizes an entry's names. The shortcode is lowercase; a canonical
// name that differs only by case becomes an alias, so :KEKW: and :kekw: both work.
func names(name, shortcode string, aliases []string) (string, string, []string, error) {
	name, shortcode = clean(name), clean(shortcode)
	if shortcode == "" {
		shortcode = name
	}
	if name == "" {
		name = shortcode
	}
	shortcode = strings.ToLower(shortcode)
	if !shortcodePattern.MatchString(shortcode) {
		return "", "", nil, fail(ErrInvalid, "Shortcodes use 2–32 letters, digits or underscores.")
	}
	if len(name) > 64 || strings.ContainsAny(name, "\r\n\x00") {
		return "", "", nil, fail(ErrInvalid, "The name is too long.")
	}
	out := []string{}
	seen := map[string]bool{shortcode: true}
	add := func(alias string) error {
		alias = clean(alias)
		if alias == "" || seen[alias] {
			return nil
		}
		if !aliasPattern.MatchString(alias) {
			return fail(ErrInvalid, "Aliases use 2–32 letters, digits, underscores or hyphens.")
		}
		seen[alias] = true
		out = append(out, alias)
		return nil
	}
	if aliasPattern.MatchString(name) {
		_ = add(name)
	}
	for _, alias := range aliases {
		if err := add(alias); err != nil {
			return "", "", nil, err
		}
	}
	if len(out) > 8 {
		return "", "", nil, fail(ErrInvalid, "An emote has at most 8 aliases.")
	}
	return name, shortcode, out, nil
}

// claim checks that no other entry uses one of the names, ignoring case, so a name always
// means exactly one emote. It returns the entries whose retired names must be released.
func (s *Service) claim(id string, wanted []string) ([]*record, error) {
	fold := map[string]string{}
	for _, n := range wanted {
		fold[strings.ToLower(n)] = n
	}
	released := []*record{}
	for _, r := range s.items {
		if r.ID == id {
			continue
		}
		for _, n := range append([]string{r.Shortcode}, r.Aliases...) {
			if wantedName, ok := fold[strings.ToLower(n)]; ok {
				return nil, fail(ErrConflict, ":%s: is already used by :%s:.", wantedName, r.Shortcode)
			}
		}
		if slices.ContainsFunc(r.Retired, func(n string) bool { _, ok := fold[strings.ToLower(n)]; return ok }) {
			released = append(released, r)
		}
	}
	return released, nil
}

func release(r *record, wanted []string) *record {
	next := *r
	next.Retired = slices.DeleteFunc(append([]string{}, r.Retired...), func(n string) bool {
		return slices.ContainsFunc(wanted, func(w string) bool { return strings.EqualFold(w, n) })
	})
	return &next
}

func random() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

type stored struct {
	key, mime string
	size      int
}

// store copies validated images into blob storage. The caller deletes them if the entry is
// not committed.
func (s *Service) store(ctx context.Context, images ...Image) ([]stored, error) {
	out := []stored{}
	for _, img := range images {
		key := random()
		if err := s.options.Blob.Put(ctx, key, bytes.NewReader(img.Data), int64(len(img.Data)), img.ContentType); err != nil {
			s.discard(out)
			return nil, fail(ErrProvider, "Emote storage is unavailable.")
		}
		out = append(out, stored{key, img.ContentType, len(img.Data)})
	}
	return out, nil
}
func (s *Service) discard(objects []stored) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, o := range objects {
		_ = s.options.Blob.Delete(ctx, o.key)
	}
}
func (s *Service) assetURL(o stored) string {
	return s.options.PublicURL + "/emotes/" + o.key + extension(o.mime)
}

// add commits a new entry with its stored images: the first image is shown, the second (if
// any) is its still frame.
func (s *Service) add(ctx context.Context, in New, e Emote, animated, still Image) (Emote, error) {
	name, shortcode, aliases, err := names(in.Name, in.Shortcode, in.Aliases)
	if err != nil {
		return Emote{}, err
	}
	images := []Image{animated}
	if still.Data != nil {
		images = append(images, still)
	}
	objects, err := s.store(ctx, images...)
	if err != nil {
		return Emote{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ProviderID != "" {
		for _, r := range s.items {
			if r.Source == e.Source && r.ProviderID == e.ProviderID {
				s.discard(objects)
				return Emote{}, fail(ErrConflict, "This %s emote is already in the catalog as :%s:.", e.Source, r.Shortcode)
			}
		}
	}
	all := append([]string{shortcode}, aliases...)
	released, err := s.claim("", all)
	if err != nil {
		s.discard(objects)
		return Emote{}, err
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	e.ID, e.Name, e.Shortcode, e.Aliases, e.Retired, e.Enabled = random()[:20], name, shortcode, aliases, []string{}, true
	e.CreatedAt, e.UpdatedAt = now, now
	e.Animated = animated.Animated
	e.Width, e.Height = animated.Width, animated.Height
	e.URL = s.assetURL(objects[0])
	e.StaticURL = e.URL
	if len(objects) > 1 {
		e.StaticURL = s.assetURL(objects[1])
	}
	if e.Creator == "" {
		e.Creator = strings.TrimSpace(in.Creator)
	}
	r := &record{Emote: e}
	for _, o := range objects {
		r.Assets = append(r.Assets, o.key)
	}
	changes := []*record{r}
	for _, old := range released {
		changes = append(changes, release(old, all))
	}
	if err = s.commit(changes, nil, in.AddedBy, objects); err != nil {
		s.discard(objects)
		return Emote{}, err
	}
	return r.copy(), nil
}

// commit writes changed and removed entries and bumps the version in one transaction, then
// updates the cache and notifies subscribers.
func (s *Service) commit(changes []*record, removed []string, addedBy string, objects []stored) error {
	tx, err := s.options.Store.DB.Begin()
	if err != nil {
		return fail(ErrProvider, "Emote storage is unavailable.")
	}
	defer tx.Rollback()
	for _, r := range changes {
		raw, _ := json.Marshal(r)
		if _, err = tx.Exec("INSERT INTO emotes(id,added_by,created_ns,payload) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", r.ID, addedBy, r.CreatedAt.UnixNano(), raw); err != nil {
			return fail(ErrProvider, "Emote storage is unavailable.")
		}
	}
	for _, id := range removed {
		if _, err = tx.Exec("DELETE FROM emotes WHERE id=?", id); err != nil {
			return fail(ErrProvider, "Emote storage is unavailable.")
		}
		if _, err = tx.Exec("DELETE FROM emote_assets WHERE emote_id=?", id); err != nil {
			return fail(ErrProvider, "Emote storage is unavailable.")
		}
	}
	for _, o := range objects {
		if _, err = tx.Exec("INSERT INTO emote_assets(object_key,emote_id,backend,mime,size) VALUES(?,?,?,?,?)", o.key, changes[0].ID, s.options.Blob.Name(), o.mime, o.size); err != nil {
			return fail(ErrProvider, "Emote storage is unavailable.")
		}
	}
	version := s.version + 1
	if _, err = tx.Exec("INSERT INTO emote_state(key,value) VALUES('version',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", strconv.FormatInt(version, 10)); err != nil {
		return fail(ErrProvider, "Emote storage is unavailable.")
	}
	if err = tx.Commit(); err != nil {
		return fail(ErrProvider, "Emote storage is unavailable.")
	}
	for _, r := range changes {
		s.items[r.ID] = r
	}
	for _, id := range removed {
		delete(s.items, id)
	}
	s.version = version
	s.notify(version)
	return nil
}

// Upload adds an image file. Animated images need a still frame for viewers who turn
// animation off: a GIF's first frame is extracted here; for other formats the browser
// supplies one.
func (s *Service) Upload(ctx context.Context, in New, data, still []byte) (Emote, error) {
	img, err := s.validate(data)
	if err != nil {
		return Emote{}, err
	}
	var frame Image
	if img.Animated {
		if len(still) > 0 {
			frame, err = s.validate(still)
			if err != nil {
				return Emote{}, err
			}
			if frame.Animated {
				return Emote{}, fail(ErrInvalid, "The still frame must not be animated.")
			}
		} else if frame, err = gifStill(img); err != nil {
			return Emote{}, err
		}
	}
	return s.add(ctx, in, Emote{Source: "upload"}, img, frame)
}

// Update renames, re-aliases, enables or disables an entry. Names it no longer has are
// retired so old messages keep rendering.
func (s *Service) Update(id string, p EmotePatch) (Emote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.items[id]
	if !ok {
		return Emote{}, ErrNotFound
	}
	next := *old
	name, shortcode, aliases := old.Name, old.Shortcode, slices.DeleteFunc(append([]string{}, old.Aliases...), func(a string) bool { return a == old.Name })
	if p.Name != nil {
		name = *p.Name
	}
	if p.Shortcode != nil {
		shortcode = *p.Shortcode
	}
	if p.Aliases != nil {
		aliases = *p.Aliases
	}
	var err error
	if next.Name, next.Shortcode, next.Aliases, err = names(name, shortcode, aliases); err != nil {
		return Emote{}, err
	}
	if p.Enabled != nil {
		next.Enabled = *p.Enabled
	}
	if p.Creator != nil {
		next.Creator = strings.TrimSpace(*p.Creator)
		if len(next.Creator) > 64 {
			return Emote{}, fail(ErrInvalid, "The attribution is too long.")
		}
	}
	all := append([]string{next.Shortcode}, next.Aliases...)
	released, err := s.claim(id, all)
	if err != nil {
		return Emote{}, err
	}
	retired := slices.DeleteFunc(append([]string{}, old.Retired...), func(n string) bool { return slices.Contains(all, n) })
	for _, n := range append([]string{old.Shortcode}, old.Aliases...) {
		if !slices.Contains(all, n) && !slices.Contains(retired, n) {
			retired = append([]string{n}, retired...)
		}
	}
	next.Retired = retired[:min(len(retired), maxRetired)]
	next.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
	changes := []*record{&next}
	for _, r := range released {
		changes = append(changes, release(r, all))
	}
	if err = s.commit(changes, nil, "", nil); err != nil {
		return Emote{}, err
	}
	return next.copy(), nil
}

// Delete removes an entry and its images. Messages that used it keep their readable
// :shortcode: text.
func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	r, ok := s.items[id]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	err := s.commit(nil, []string{id}, "", nil)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	for _, key := range r.Assets {
		_ = s.options.Blob.Delete(ctx, key)
	}
	return nil
}

// Open streams a stored image by its object key.
func (s *Service) Open(ctx context.Context, key string) (string, io.ReadCloser, error) {
	var mime, backend string
	if err := s.options.Store.DB.QueryRowContext(ctx, "SELECT mime,backend FROM emote_assets WHERE object_key=?", key).Scan(&mime, &backend); err != nil || backend != s.options.Blob.Name() {
		return "", nil, ErrNotFound
	}
	body, err := s.options.Blob.Open(ctx, key)
	if err != nil {
		return "", nil, ErrNotFound
	}
	return mime, body, nil
}
