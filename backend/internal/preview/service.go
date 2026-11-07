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
	key := u.String()
	s.mu.Lock()
	cached, ok := s.cache[key]
	s.mu.Unlock()
	if ok && time.Now().Before(cached.until) {
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
		value, err := parse(io.LimitReader(response.Body, 512<<10), u)
		response.Body.Close()
		if err != nil {
			return model.LinkPreview{}, err
		}
		if value.Image != "" {
			if _, err = s.validate(ctx, value.Image); err != nil {
				value.Image = ""
			}
		}
		s.mu.Lock()
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
		s.mu.Unlock()
		return value, nil
	}
	return model.LinkPreview{}, errors.New("too many preview redirects")
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
func parse(body io.Reader, base *url.URL) (model.LinkPreview, error) {
	result := model.LinkPreview{URL: base.String(), SiteName: base.Hostname()}
	tokens := html.NewTokenizer(body)
	tokens.SetMaxBuf(64 << 10)
	inTitle := false
	var title strings.Builder
	for {
		kind := tokens.Next()
		switch kind {
		case html.ErrorToken:
			if tokens.Err() != io.EOF {
				return result, errors.New("preview HTML exceeded its parsing limit")
			}
			if result.Title == "" {
				result.Title = bounded(title.String(), 200)
			}
			if result.Title == "" && result.Description == "" {
				return result, errors.New("page has no preview metadata")
			}
			return result, nil
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
				if result.Title == "" {
					result.Title = bounded(title.String(), 200)
				}
				if result.Title != "" || result.Description != "" {
					return result, nil
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
			key, content := "", ""
			for _, a := range token.Attr {
				switch a.Key {
				case "property", "name":
					key = strings.ToLower(a.Val)
				case "content":
					content = a.Val
				}
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
			case "og:image", "twitter:image":
				if result.Image == "" {
					if image, err := url.Parse(content); err == nil {
						result.Image = base.ResolveReference(image).String()
					}
				}
			}
		}
	}
}

// SafeHTTPClient provides public-address-pinned, credential-free HTTPS fetching.
func SafeHTTPClient() *http.Client { return New(Options{}).client }
