// Package preview fetches inert public-page metadata. It never renders HTML or
// forwards account cookies, IRC credentials, or HTTP authorization headers.
package preview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/net/html"
	"ircord/backend/internal/model"
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type Options struct {
	Client   *http.Client
	Resolver Resolver
}
type entry struct {
	value model.LinkPreview
	until time.Time
}

func (e entry) fresh() bool { return time.Now().Before(e.until) }

type Service struct {
	client   *http.Client
	resolver Resolver
	mu       sync.Mutex
	cache    map[string]entry
	slots    chan struct{}
}

func New(options Options) *Service {
	resolver := options.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{Proxy: nil, MaxIdleConns: 20, MaxIdleConnsPerHost: 2, MaxResponseHeaderBytes: 32768, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := resolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, errors.New("host has no addresses")
			}
			for _, ip := range ips {
				if !public(ip) {
					return nil, errors.New("private destination blocked")
				}
			}
			for _, ip := range ips {
				conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
			}
			return nil, errors.New("preview connection failed")
		}}}
	}
	copy := *client
	copy.Jar = nil
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Service{client: &copy, resolver: resolver, cache: map[string]entry{}, slots: make(chan struct{}, 4)}
}

// maxPage bounds how much of a page is read. Pages such as news front pages and video sites
// inline hundreds of kilobytes of script before their metadata, so the parser's token buffer
// matches this bound instead of rejecting a single large script.
const maxPage = 1 << 20

// gifProviders are share-page sites whose pages name an animated GIF that is shown inline
// instead of a text card. The GIF itself must be served from the same site or its subdomains.
var gifProviders = []string{"tenor.com", "giphy.com"}

func within(host, domain string) bool {
	host = strings.ToLower(host)
	return host == domain || strings.HasSuffix(host, "."+domain)
}
func gifProvider(host string) string {
	for _, domain := range gifProviders {
		if within(host, domain) {
			return domain
		}
	}
	return ""
}

var denied = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("2002::/16")}

func public(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range denied {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func (s *Service) validate(ctx context.Context, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || len(raw) > 2048 {
		return nil, errors.New("preview requires a public HTTPS URL")
	}
	u.Fragment = ""
	addresses, err := s.resolver.LookupNetIP(ctx, "ip", u.Hostname())
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("preview host is unavailable")
	}
	for _, ip := range addresses {
		if !public(ip) {
			return nil, errors.New("private or reserved preview destinations are blocked")
		}
	}
	return u, nil
}
func (s *Service) Fetch(ctx context.Context, raw string) (model.LinkPreview, error) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return model.LinkPreview{}, errors.New("preview capacity reached; try again shortly")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	u, err := s.validate(ctx, raw)
	if err != nil {
		return model.LinkPreview{}, err
	}
	if video, ok := ParseYouTube(u); ok {
		return s.youtube(ctx, u.String(), video)
	}
	key := u.String()
	s.mu.Lock()
	cached, ok := s.cache[key]
	s.mu.Unlock()
	if ok && cached.fresh() {
		return cached.value, nil
	}
	for redirects := 0; redirects <= 3; redirects++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("User-Agent", "IRCord-LinkPreview/1")
		response, err := s.client.Do(req)
		if err != nil {
			return model.LinkPreview{}, errors.New("preview request failed")
		}
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			location, err := response.Location()
			response.Body.Close()
			if err != nil {
				return model.LinkPreview{}, errors.New("invalid preview redirect")
			}
			u, err = s.validate(ctx, location.String())
			if err != nil {
				return model.LinkPreview{}, err
			}
			// A short link that forwards to YouTube takes the oEmbed path like a direct link.
			if video, ok := ParseYouTube(u); ok {
				return s.youtube(ctx, raw, video)
			}
			continue
		}
		if response.StatusCode != 200 {
			response.Body.Close()
			return model.LinkPreview{}, fmt.Errorf("preview returned HTTP %d", response.StatusCode)
		}
		contentType := strings.ToLower(strings.Split(response.Header.Get("Content-Type"), ";")[0])
		if contentType != "text/html" && contentType != "application/xhtml+xml" {
			response.Body.Close()
			return model.LinkPreview{}, errors.New("preview is not an HTML page")
		}
		value, refresh, err := parse(io.LimitReader(response.Body, maxPage), u)
		response.Body.Close()
		if err != nil && refresh != "" {
			// A metadata-less stub that forwards to the real page (GIPHY serves one for every
			// non-canonical share URL) is followed like a redirect, within the same budget.
			target, parseErr := url.Parse(refresh)
			if parseErr != nil {
				return model.LinkPreview{}, err
			}
			next := u.ResolveReference(target)
			if !strings.EqualFold(next.Hostname(), u.Hostname()) {
				return model.LinkPreview{}, err
			}
			if u, err = s.validate(ctx, next.String()); err != nil {
				return model.LinkPreview{}, err
			}
			if video, ok := ParseYouTube(u); ok {
				return s.youtube(ctx, raw, video)
			}
			continue
		}
		if err != nil {
			return model.LinkPreview{}, err
		}
		if value.Image != "" {
			if _, err = s.validate(ctx, value.Image); err != nil {
				value.Image, value.ImageWidth, value.ImageHeight = "", 0, 0
			}
		}
		// Gallery images are validated like the lead image; the gallery is dropped when fewer
		// than two remain.
		images := value.Images[:0]
		for _, item := range value.Images {
			if _, err = s.validate(ctx, item.URL); err == nil {
				images = append(images, item)
			}
		}
		value.Images = images
		if len(value.Images) < 2 {
			value.Images = nil
		}
		if value.Media != nil {
			if _, err = s.validate(ctx, value.Media.URL); err != nil {
				value.Media = nil
			}
		}
		s.store(key, value)
		return value, nil
	}
	return model.LinkPreview{}, errors.New("too many preview redirects")
}

// store caches a preview for ten minutes, evicting the oldest entry past 256.
func (s *Service) store(key string, value model.LinkPreview) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cache) >= 256 {
		var oldestKey string
		var oldest time.Time
		for key, item := range s.cache {
			if oldestKey == "" || item.until.Before(oldest) {
				oldestKey, oldest = key, item.until
			}
		}
		delete(s.cache, oldestKey)
	}
	s.cache[key] = entry{value, time.Now().Add(10 * time.Minute)}
}
func bounded(text string, limit int) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(text) {
		if unicode.IsControl(r) {
			if r == '\n' || r == '\t' {
				r = ' '
			} else {
				continue
			}
		}
		if n >= limit {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// image is one og:image entry with the structured properties that follow it.
type image struct {
	url, kind     string
	width, height int
}

// media picks the inline GIF a supported share page names, or nil for every other page.
func media(base *url.URL, images []image) *model.PreviewMedia {
	domain := gifProvider(base.Hostname())
	if domain == "" {
		return nil
	}
	for _, candidate := range images {
		u, err := url.Parse(candidate.url)
		if err != nil || u.Scheme != "https" || !within(u.Hostname(), domain) {
			continue
		}
		if candidate.kind == "image/gif" || (candidate.kind == "" && strings.HasSuffix(strings.ToLower(u.Path), ".gif")) {
			return &model.PreviewMedia{URL: u.String(), Kind: "gif", Width: candidate.width, Height: candidate.height}
		}
	}
	return nil
}

// maxGallery bounds how many images of a gallery page are passed on.
const maxGallery = 10

// albumPath recognizes album and gallery pages by their URL: a path segment such as /album/,
// /gallery/, /a/ (Imgur) or /sets/ (Flickr). Only such pages become galleries; any other page
// declaring several og:images (one picture at several sizes, an avatar and a banner) keeps its
// ordinary card.
var albumPath = regexp.MustCompile(`(?i)(^|/)(album|albums|gallery|galleries|a|sets)/[^/]`)

// classify records the page kind, the lead image's declared size and, for album pages declaring
// several distinct images, the gallery.
func classify(base *url.URL, result *model.LinkPreview, pageType string, articleTags bool, images []image) {
	if pageType == "article" || articleTags {
		result.Kind = "article"
	}
	for _, item := range images {
		if item.url == result.Image {
			result.ImageWidth, result.ImageHeight = item.width, item.height
			break
		}
	}
	// Articles commonly declare one lead image at several sizes; that is not a gallery.
	if result.Media != nil || result.Kind == "article" || !albumPath.MatchString(base.Path) {
		return
	}
	seen := map[string]bool{}
	var gallery []model.PreviewImage
	for _, item := range images {
		u, err := url.Parse(item.url)
		if err != nil || u.Scheme != "https" || seen[item.url] || len(gallery) >= maxGallery {
			continue
		}
		seen[item.url] = true
		gallery = append(gallery, model.PreviewImage{URL: item.url, Width: item.width, Height: item.height})
	}
	if len(gallery) >= 2 {
		result.Images = gallery
	}
}

// refreshTarget returns the URL of an immediate `<meta http-equiv="refresh">`, or "".
func refreshTarget(content string) string {
	delay, rest, found := strings.Cut(content, ";")
	if !found || strings.TrimSpace(delay) != "0" {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if len(rest) < 4 || !strings.EqualFold(rest[:4], "url=") {
		return ""
	}
	return strings.Trim(strings.TrimSpace(rest[4:]), `"'`)
}
func dimension(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 || n > 10000 {
		return 0
	}
	return n
}

// parse reads a page's preview metadata. When the page has none, refresh is the target of an
// immediate meta refresh it declares, if any.
func parse(body io.Reader, base *url.URL) (_ model.LinkPreview, refresh string, _ error) {
	result := model.LinkPreview{URL: base.String(), SiteName: base.Hostname()}
	tokens := html.NewTokenizer(body)
	tokens.SetMaxBuf(maxPage)
	inTitle := false
	var title strings.Builder
	var images []image
	// skipping is set while the structured properties of an og:image past the cap arrive.
	skipping := false
	pageType, articleTags := "", false
	done := func() (model.LinkPreview, string, error) {
		if result.Title == "" {
			result.Title = bounded(title.String(), 200)
		}
		result.Media = media(base, images)
		classify(base, &result, pageType, articleTags, images)
		if result.Title == "" && result.Media != nil {
			// The client only shows previews with a title; a bare GIF page still gets one.
			result.Title = bounded(result.SiteName+" GIF", 200)
		}
		if result.Title == "" && result.Description == "" && result.Media == nil {
			return result, refresh, errors.New("page has no preview metadata")
		}
		return result, "", nil
	}
	for {
		kind := tokens.Next()
		switch kind {
		case html.ErrorToken:
			if tokens.Err() != io.EOF {
				return result, "", errors.New("preview HTML exceeded its parsing limit")
			}
			return done()
		case html.TextToken:
			if inTitle && title.Len() < 1000 {
				title.Write(tokens.Text())
			}
		case html.EndTagToken:
			token := tokens.Token()
			if token.Data == "title" {
				inTitle = false
			}
			if token.Data == "head" {
				if value, _, err := done(); err == nil {
					return value, "", nil
				}
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokens.Token()
			if token.Data == "title" {
				inTitle = true
			}
			if token.Data != "meta" {
				continue
			}
			key, content, equiv := "", "", ""
			for _, a := range token.Attr {
				switch a.Key {
				case "property", "name":
					key = strings.ToLower(a.Val)
				case "content":
					content = a.Val
				case "http-equiv":
					equiv = strings.ToLower(a.Val)
				}
			}
			if equiv == "refresh" && refresh == "" {
				refresh = refreshTarget(content)
			}
			switch key {
			case "og:title", "twitter:title":
				if result.Title == "" {
					result.Title = bounded(content, 200)
				}
			case "og:description", "description", "twitter:description":
				if result.Description == "" {
					result.Description = bounded(content, 500)
				}
			case "og:site_name":
				result.SiteName = bounded(content, 100)
			case "og:type":
				pageType = strings.ToLower(strings.TrimSpace(content))
			case "article:published_time", "article:author", "article:publisher":
				articleTags = true
			case "og:image", "og:image:url", "og:image:secure_url", "twitter:image":
				reference, err := url.Parse(strings.TrimSpace(content))
				if err != nil {
					continue
				}
				resolved := base.ResolveReference(reference).String()
				if result.Image == "" {
					result.Image = resolved
				}
				if key == "og:image" || key == "og:image:url" {
					skipping = len(images) >= 16
					if !skipping {
						images = append(images, image{url: resolved})
					}
				}
			case "og:image:type", "og:image:width", "og:image:height":
				if len(images) == 0 || skipping {
					continue
				}
				last := &images[len(images)-1]
				switch key {
				case "og:image:type":
					last.kind = strings.ToLower(strings.TrimSpace(content))
				case "og:image:width":
					last.width = dimension(content)
				case "og:image:height":
					last.height = dimension(content)
				}
			}
		}
	}
}

// SafeHTTPClient provides public-address-pinned, credential-free HTTPS fetching.
func SafeHTTPClient() *http.Client { return New(Options{}).client }
