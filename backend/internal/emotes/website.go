package emotes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

var websiteSlug = regexp.MustCompile(`^[0-9]{1,12}[-_][A-Za-z0-9_-]{1,98}$`)
var websiteTag = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

type websitePage struct {
	body       []byte
	fetched    time.Time
	generation string
}
type websitePending struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	page    websitePage
	err     error
}
type websiteCachedEntry struct {
	entry   discoveryEntry
	expires time.Time
}
type websiteCache struct {
	mu         sync.Mutex
	pages      map[string]websitePage
	pending    map[string]*websitePending
	entries    map[string]websiteCachedEntry
	slots      chan struct{}
	retryAfter map[string]time.Time
}

// websiteHTML caches only bounded, provider-origin HTML; simultaneous requests share a fetch.
func (s *Service) websiteHTML(ctx context.Context, key string) (websitePage, bool, error) {
	s.website.mu.Lock()
	if s.website.pages == nil {
		s.website.pages = map[string]websitePage{}
		s.website.pending = map[string]*websitePending{}
		s.website.entries = map[string]websiteCachedEntry{}
		s.website.slots = make(chan struct{}, 4)
		s.website.retryAfter = map[string]time.Time{}
	}
	old, exists := s.website.pages[key]
	if exists && time.Since(old.fetched) < 5*time.Minute {
		s.website.mu.Unlock()
		return old, false, nil
	}
	if until := s.website.retryAfter[key]; time.Now().Before(until) {
		s.website.mu.Unlock()
		if exists && time.Since(old.fetched) < time.Hour {
			return old, true, nil
		}
		return websitePage{}, false, fail(ErrProvider, "Emoji search is unavailable. Try again later.")
	}
	p, ok := s.website.pending[key]
	if !ok {
		if len(s.website.pending) >= 32 {
			s.website.mu.Unlock()
			return websitePage{}, false, fail(ErrProvider, "The emoji search is busy. Try again shortly.")
		}
		fetchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		p = &websitePending{done: make(chan struct{}), cancel: cancel}
		s.website.pending[key] = p
		slots := s.website.slots
		go func() {
			defer cancel()
			var body []byte
			var err error
			select {
			case slots <- struct{}{}:
				body, err = s.fetchWebsite(fetchCtx, key)
				<-slots
			case <-fetchCtx.Done():
				err = fetchCtx.Err()
			}
			page := websitePage{body: body, fetched: time.Now()}
			if err == nil {
				hash := sha256.Sum256(append(append([]byte{}, body...), []byte(page.fetched.Format(time.RFC3339Nano))...))
				page.generation = hex.EncodeToString(hash[:12])
			}
			s.website.mu.Lock()
			if err == nil {
				delete(s.website.retryAfter, key)
				if len(s.website.pages) >= 32 {
					for k := range s.website.pages {
						if k != key {
							delete(s.website.pages, k)
							break
						}
					}
				}
				s.website.pages[key] = page
			} else {
				if !errors.Is(fetchCtx.Err(), context.Canceled) {
					if len(s.website.retryAfter) >= 32 {
						for k := range s.website.retryAfter {
							delete(s.website.retryAfter, k)
							break
						}
					}
					s.website.retryAfter[key] = time.Now().Add(time.Minute)
				}
				if old, ok := s.website.pages[key]; ok && time.Since(old.fetched) < time.Hour {
					page = old
					err = nil
				}
			}
			p.page, p.err = page, err
			if s.website.pending[key] == p {
				delete(s.website.pending, key)
			}
			close(p.done)
			s.website.mu.Unlock()
		}()
	}
	p.waiters++
	s.website.mu.Unlock()
	defer func() {
		s.website.mu.Lock()
		p.waiters--
		if p.waiters == 0 && s.website.pending[key] == p {
			delete(s.website.pending, key)
			p.cancel()
		}
		s.website.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return websitePage{}, false, ctx.Err()
	case <-p.done:
		if p.err != nil {
			return websitePage{}, false, fail(ErrProvider, "Emoji search is unavailable. Try again later.")
		}
		return p.page, time.Since(p.page.fetched) >= 5*time.Minute, nil
	}
}

// fetchWebsite accepts only paths constructed internally; redirects are returned to the
// search caller for strict same-origin validation, never followed automatically.
func (s *Service) fetchWebsite(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.options.Endpoints.EmojiGGWeb+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "IRCord emote catalog")
	res, err := s.options.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusFound {
		return []byte("redirect:" + res.Header.Get("Location")), nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider status %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return nil, errors.New("invalid website response")
	}
	if !validWebsiteDocument(body, strings.HasPrefix(path, "/emoji/")) {
		return nil, errors.New("unrecognized website response")
	}
	return body, nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, name string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == name {
			return true
		}
	}
	return false
}
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
func nodeText(n *html.Node) string {
	var b strings.Builder
	walk(n, func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
	})
	return strings.TrimSpace(b.String())
}
func parseHTML(body []byte) (*html.Node, error) {
	if len(body) == 0 || len(body) > 1<<20 {
		return nil, errors.New("invalid HTML")
	}
	return html.Parse(bytes.NewReader(body))
}

func validWebsiteDocument(body []byte, detail bool) bool {
	root, err := parseHTML(body)
	if err != nil {
		return false
	}
	items, cards, empty, canonical, preview := false, false, false, false, false
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if attr(n, "id") == "items" {
			items = true
		}
		if hasClass(n, "asset-card") {
			cards = true
		}
		if (n.Data == "h3" || n.Data == "h2") && strings.EqualFold(nodeText(n), "No results found") {
			empty = true
		}
		if n.Data == "link" && attr(n, "rel") == "canonical" {
			canonical = true
		}
		if n.Data == "img" && attr(n, "id") == "emojiPreview" {
			preview = true
		}
	})
	if detail {
		return canonical && preview
	}
	return items && (cards || empty)
}

type websiteParsed struct {
	entries []discoveryEntry
	next    bool
}

func (s *Service) parseWebsitePage(body []byte, query, tag string, page int) (websiteParsed, error) {
	root, err := parseHTML(body)
	if err != nil {
		return websiteParsed{}, err
	}
	var result websiteParsed
	itemsSeen, empty := false, false
	nextHrefs := []string{}
	seen := map[string]bool{}
	base := s.options.Endpoints.EmojiGGWeb
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if attr(n, "id") == "items" {
			itemsSeen = true
		}
		if (n.Data == "h3" || n.Data == "h2") && strings.EqualFold(nodeText(n), "No results found") {
			empty = true
		}
		if n.Data == "a" && hasClass(n, "next-page") {
			nextHrefs = append(nextHrefs, attr(n, "href"))
		}
		if !hasClass(n, "asset-card") {
			return
		}
		var image, slug, name, creator string
		walk(n, func(x *html.Node) {
			if x.Type != html.ElementNode {
				return
			}
			if x.Data == "img" && attr(x, "data-src") != "" {
				image = attr(x, "data-src")
			}
			if x.Data == "a" {
				h := attr(x, "href")
				if strings.HasPrefix(h, base+"/emoji/") {
					candidate := strings.TrimPrefix(h, base+"/emoji/")
					if websiteSlug.MatchString(candidate) {
						slug = candidate
						if text := nodeText(x); text != "" && text != "View" {
							name = text
						}
					}
				}
				if strings.HasPrefix(h, base+"/user/") {
					creator = nodeText(x)
				}
			}
		})
		if !websiteSlug.MatchString(slug) || len(slug) > 100 || name == "" || len(name) > 64 || strings.ContainsAny(name, "\r\n\x00") {
			return
		}
		if seen[slug] {
			return
		}
		expected := s.options.Endpoints.EmojiGGCDN + slug
		if image != expected+".png" && image != expected+".gif" {
			return
		}
		item := DiscoveryItem{Provider: "emoji.gg", ProviderID: slug, Name: name, Creator: short(creator), SourceURL: base + "/emoji/" + slug, PreviewURL: "/api/v1/emotes/discover/emoji.gg/" + slug + "/preview", Animated: strings.HasSuffix(image, ".gif")}
		meta := ProviderEmote{Provider: "emoji.gg", ID: slug, Name: name, Creator: item.Creator, SourceURL: item.SourceURL, ImageURL: image}
		seen[slug] = true
		result.entries = append(result.entries, discoveryEntry{item, meta})
	})
	if !itemsSeen || len(result.entries) > 50 || len(result.entries) == 0 && !empty {
		return websiteParsed{}, errors.New("unrecognized search page")
	}
	if !empty && len(nextHrefs) > 0 {
		valid := false
		for _, h := range nextHrefs {
			u, e := url.Parse(h)
			if e != nil || u.User != nil || u.Scheme+"://"+u.Host != base || u.Fragment != "" {
				continue
			}
			if tag != "" {
				valid = h == base+"/emojis/"+tag+"&page="+strconv.Itoa(page+1)+"&sort=recent"
			} else if query == "" {
				values := u.Query()
				valid = u.Path == "/" && values.Get("page") == strconv.Itoa(page+1) && values.Get("sort") == "featured" && len(values) == 2
			} else {
				values := u.Query()
				valid = u.Path == "/search" && values.Get("q") == query && values.Get("page") == strconv.Itoa(page+1) && len(values) == 2
			}
			if valid {
				break
			}
		}
		if !valid {
			return websiteParsed{}, errors.New("unrecognized next page")
		}
		result.next = true
	}
	return result, nil
}

type websiteCursor struct {
	Binding    string `json:"b"`
	Origin     string `json:"o"`
	Generation string `json:"g"`
	Page       int    `json:"p"`
	Offset     int    `json:"i"`
	Tag        string `json:"t"`
}

func websiteKey(query, tag string, page int) string {
	if tag != "" {
		if page == 0 {
			return "/emojis/" + tag
		}
		return "/emojis/" + tag + "&page=" + strconv.Itoa(page) + "&sort=recent"
	}
	if query == "" {
		if page == 0 {
			return "/"
		}
		return "/?page=" + strconv.Itoa(page) + "&sort=featured"
	}
	v := url.Values{"q": {query}}
	if page > 0 {
		v.Set("page", strconv.Itoa(page))
	}
	return "/search?" + v.Encode()
}

func websiteOrigin(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:12])
}

func (s *Service) discoverWebsite(ctx context.Context, q DiscoveryQuery) (DiscoveryResult, error) {
	if strings.ContainsAny(q.Query, "\x00\r\n") {
		return DiscoveryResult{}, fail(ErrInvalid, "Invalid search query.")
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", q.Query, q.Animation, q.Limit)))
	binding := hex.EncodeToString(h[:12])
	cur := websiteCursor{Binding: binding}
	if q.Cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil || json.Unmarshal(raw, &cur) != nil || cur.Binding != binding || cur.Page < 0 || cur.Page > 10000 || cur.Offset < 0 || cur.Offset > 50 || cur.Tag != "" && !websiteTag.MatchString(cur.Tag) {
			return DiscoveryResult{}, fail(ErrInvalid, "Invalid discovery cursor.")
		}
		s.website.mu.Lock()
		var prev websitePage
		ok := false
		for key, candidate := range s.website.pages {
			if websiteOrigin(key) == cur.Origin {
				prev, ok = candidate, true
				break
			}
		}
		s.website.mu.Unlock()
		if !ok || prev.generation != cur.Generation || time.Since(prev.fetched) >= time.Hour {
			return DiscoveryResult{}, fail(ErrInvalid, "Discovery cursor expired.")
		}
	}
	key := websiteKey(q.Query, cur.Tag, cur.Page)
	cached, stale, err := s.websiteHTML(ctx, key)
	if err != nil {
		return DiscoveryResult{}, err
	}
	if bytes.HasPrefix(cached.body, []byte("redirect:")) {
		if cur.Page != 0 || cur.Tag != "" {
			return DiscoveryResult{}, fail(ErrProvider, "The emoji search returned an unexpected redirect.")
		}
		u, e := url.Parse(strings.TrimPrefix(string(cached.body), "redirect:"))
		base := s.options.Endpoints.EmojiGGWeb
		if e != nil || u.User != nil || u.Scheme+"://"+u.Host != base || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/emojis/") {
			return DiscoveryResult{}, fail(ErrProvider, "The emoji search returned an unexpected redirect.")
		}
		cur.Tag = strings.TrimPrefix(u.Path, "/emojis/")
		if !websiteTag.MatchString(cur.Tag) {
			return DiscoveryResult{}, fail(ErrProvider, "The emoji search returned an unexpected redirect.")
		}
		key = websiteKey(q.Query, cur.Tag, 0)
		cached, stale, err = s.websiteHTML(ctx, key)
		if err != nil {
			return DiscoveryResult{}, err
		}
	}
	parsed, err := s.parseWebsitePage(cached.body, q.Query, cur.Tag, cur.Page)
	if err != nil {
		return DiscoveryResult{}, fail(ErrProvider, "The emoji search returned unusable data.")
	}
	if cur.Offset > len(parsed.entries) {
		return DiscoveryResult{}, fail(ErrInvalid, "Invalid discovery cursor.")
	}
	s.website.mu.Lock()
	for _, entry := range parsed.entries {
		if len(s.website.entries) >= 512 {
			for k := range s.website.entries {
				delete(s.website.entries, k)
				break
			}
		}
		s.website.entries[entry.item.ProviderID] = websiteCachedEntry{entry, time.Now().Add(time.Hour)}
	}
	s.website.mu.Unlock()
	s.mu.Lock()
	installed := map[string]bool{}
	for _, r := range s.items {
		if r.Source == "emoji.gg" {
			installed[r.ProviderID] = true
		}
	}
	s.mu.Unlock()
	out := DiscoveryResult{Items: []DiscoveryItem{}, Filters: DiscoveryFilters{Sources: []string{"emoji.gg"}, Categories: []DiscoveryCategory{}, Animations: []string{"all", "static", "animated"}}, Stale: stale}
	index := cur.Offset
	for index < len(parsed.entries) {
		item := parsed.entries[index].item
		if q.Animation == "static" && item.Animated || q.Animation == "animated" && !item.Animated {
			index++
			continue
		}
		if len(out.Items) >= q.Limit {
			break
		}
		item.Installed = installed[item.ProviderID]
		out.Items = append(out.Items, item)
		index++
	}
	if index < len(parsed.entries) || parsed.next && cur.Page < 10000 {
		next := websiteCursor{Binding: binding, Origin: websiteOrigin(key), Generation: cached.generation, Page: cur.Page, Offset: index, Tag: cur.Tag}
		if index == len(parsed.entries) {
			next.Page++
			next.Offset = 0
		}
		raw, _ := json.Marshal(next)
		out.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, nil
}

func (s *Service) websiteEntry(id string) (discoveryEntry, bool) {
	s.website.mu.Lock()
	defer s.website.mu.Unlock()
	v, ok := s.website.entries[id]
	return v.entry, ok && time.Now().Before(v.expires)
}

func (s *Service) websiteDetail(ctx context.Context, id string) (ProviderEmote, error) {
	if !websiteSlug.MatchString(id) || len(id) > 100 {
		return ProviderEmote{}, fail(ErrInvalid, "Invalid emoji.gg ID.")
	}
	page, _, err := s.websiteHTML(ctx, "/emoji/"+id)
	if err != nil {
		return ProviderEmote{}, err
	}
	root, err := parseHTML(page.body)
	if err != nil {
		return ProviderEmote{}, fail(ErrProvider, "The emoji details are unavailable.")
	}
	base := s.options.Endpoints.EmojiGGWeb
	var canonical, image, name, creator, license string
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if n.Data == "link" && attr(n, "rel") == "canonical" {
			canonical = attr(n, "href")
		}
		if n.Data == "img" && attr(n, "id") == "emojiPreview" {
			image = attr(n, "src")
			name = strings.TrimSuffix(attr(n, "alt"), " Emoji")
		}
		if creator == "" && n.Data == "a" && strings.HasPrefix(attr(n, "href"), base+"/user/") {
			creator = nodeText(n)
		}
		if license == "" && n.Data == "h5" && strings.Contains(nodeText(n), "License") {
			license = nodeText(n)
		}
	})
	expected := s.options.Endpoints.EmojiGGCDN + id
	if canonical != base+"/emoji/"+id || image != expected+".png" && image != expected+".gif" || name == "" || len(name) > 64 || strings.ContainsAny(name, "\r\n\x00") || len(license) > 64 {
		return ProviderEmote{}, fail(ErrProvider, "The emoji details are unusable.")
	}
	if creator == "" {
		if entry, ok := s.websiteEntry(id); ok && entry.meta.ImageURL == image {
			creator = entry.meta.Creator
		}
	}
	return ProviderEmote{Provider: "emoji.gg", ID: id, Name: name, Creator: short(creator), SourceURL: canonical, ImageURL: image, License: license}, nil
}
