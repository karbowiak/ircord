// Package iframely is a server-side client for the self-hosted Iframely APIs.
// Returned HTML is untrusted provider data, not safe application markup.
package iframely

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	parse "golang.org/x/net/html"
)

type Options struct {
	BaseURL, APIKey  string
	HTTPClient       *http.Client
	MaxResponseBytes int64
}

type Client struct {
	base  *url.URL
	key   string
	http  *http.Client
	limit int64
}

type RequestOptions struct {
	MaxWidth, MaxHeight int
	Refresh             bool
}

type Meta struct {
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Author      string  `json:"author,omitempty"`
	AuthorURL   string  `json:"author_url,omitempty"`
	Site        string  `json:"site,omitempty"`
	Canonical   string  `json:"canonical,omitempty"`
	Medium      string  `json:"medium,omitempty"`
	Duration    float64 `json:"duration,omitempty"`
	Date        string  `json:"date,omitempty"`
}

type Link struct {
	Href  string                     `json:"href,omitempty"`
	Type  string                     `json:"type,omitempty"`
	Rel   []string                   `json:"rel,omitempty"`
	HTML  string                     `json:"html,omitempty"`
	Media map[string]json.RawMessage `json:"media,omitempty"`
}

// Links accepts both the grouped API representation and the flat link list.
type Links map[string][]Link

func (l *Links) UnmarshalJSON(data []byte) error {
	if strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		var flat []Link
		if err := json.Unmarshal(data, &flat); err != nil {
			return err
		}
		*l = Links{}
		for _, link := range flat {
			key := "other"
			if len(link.Rel) > 0 {
				key = link.Rel[0]
			}
			(*l)[key] = append((*l)[key], link)
		}
		return nil
	}
	type grouped Links
	return json.Unmarshal(data, (*grouped)(l))
}

type Response struct {
	ID    string   `json:"id,omitempty"`
	URL   string   `json:"url,omitempty"`
	HTML  string   `json:"html,omitempty"`
	Rel   []string `json:"rel,omitempty"`
	Meta  Meta     `json:"meta"`
	Links Links    `json:"links,omitempty"`
}

type OEmbed struct {
	Type            string `json:"type"`
	Version         string `json:"version"`
	URL             string `json:"url,omitempty"`
	Title           string `json:"title,omitempty"`
	Description     string `json:"description,omitempty"`
	AuthorName      string `json:"author_name,omitempty"`
	Author          string `json:"author,omitempty"`
	AuthorURL       string `json:"author_url,omitempty"`
	ProviderName    string `json:"provider_name,omitempty"`
	ProviderURL     string `json:"provider_url,omitempty"`
	HTML            string `json:"html,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	ThumbnailURL    string `json:"thumbnail_url,omitempty"`
	ThumbnailWidth  int    `json:"thumbnail_width,omitempty"`
	ThumbnailHeight int    `json:"thumbnail_height,omitempty"`
}

type APIError struct {
	Status int
	Code   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Iframely request failed (HTTP %d, code %s)", e.Status, e.Code)
}

func New(o Options) (*Client, error) {
	u, err := url.Parse(o.BaseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid Iframely base URL")
	}
	// Also accept the endpoint URL used by existing deployments.
	u.Path = strings.TrimRight(u.Path, "/")
	for _, suffix := range []string{"/iframely", "/oembed"} {
		u.Path = strings.TrimSuffix(u.Path, suffix)
	}
	h := http.Client{Timeout: 15 * time.Second}
	if o.HTTPClient != nil {
		h = *o.HTTPClient
	}
	if h.Timeout == 0 {
		h.Timeout = 15 * time.Second
	}
	// API keys belong only to the configured service, including during redirects.
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if o.MaxResponseBytes <= 0 {
		o.MaxResponseBytes = 2 << 20
	}
	return &Client{base: u, key: o.APIKey, http: &h, limit: o.MaxResponseBytes}, nil
}

func (c *Client) Lookup(ctx context.Context, raw string, opts RequestOptions) (Response, error) {
	var out Response
	err := c.get(ctx, "iframely", raw, opts, &out)
	if err == nil && relativeFrame(out.HTML) {
		// Some inferred embed URLs (notably Bluesky's /render URL) lose
		// their provider origin. The oEmbed endpoint retains the publisher's
		// original widget instead. Keep metadata if that fallback is unavailable.
		oembed, fallbackErr := c.OEmbed(ctx, raw, opts)
		out.HTML = ""
		if fallbackErr == nil && !relativeFrame(oembed.HTML) {
			out.HTML = oembed.HTML
		}
		for key, links := range out.Links {
			for index := range links {
				if relativeFrame(links[index].HTML) {
					links[index].HTML = ""
				}
			}
			out.Links[key] = links
		}
	}
	return out, err
}

func relativeFrame(markup string) bool {
	tokens := parse.NewTokenizer(strings.NewReader(markup))
	for {
		kind := tokens.Next()
		if kind == parse.ErrorToken {
			return false
		}
		if kind != parse.StartTagToken && kind != parse.SelfClosingTagToken {
			continue
		}
		tag := tokens.Token()
		if tag.Data != "iframe" {
			continue
		}
		for _, attr := range tag.Attr {
			if attr.Key == "src" && attr.Val != "" {
				u, err := url.Parse(attr.Val)
				if err == nil && u.Scheme == "" && u.Host == "" {
					return true
				}
			}
		}
	}
}

func (c *Client) OEmbed(ctx context.Context, raw string, opts RequestOptions) (OEmbed, error) {
	var out OEmbed
	err := c.get(ctx, "oembed", raw, opts, &out)
	return out, err
}

func (c *Client) get(ctx context.Context, endpoint, raw string, opts RequestOptions, out any) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || len(raw) > 8192 {
		return errors.New("invalid Iframely target URL")
	}
	api := *c.base
	api.Path = strings.TrimRight(api.Path, "/") + "/" + endpoint
	query := url.Values{"url": {raw}}
	if c.key != "" {
		query.Set("api_key", c.key)
	}
	if opts.MaxWidth > 0 {
		query.Set("maxwidth", fmt.Sprint(opts.MaxWidth))
	}
	if opts.MaxHeight > 0 {
		query.Set("maxheight", fmt.Sprint(opts.MaxHeight))
	}
	if opts.Refresh {
		query.Set("refresh", "1")
	}
	api.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api.String(), nil)
	if err != nil {
		return errors.New("invalid Iframely request")
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("Iframely service unavailable") // never expose the key in a url.Error
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, c.limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > c.limit {
		return errors.New("Iframely response exceeds limit")
	}
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	_ = json.Unmarshal(data, &envelope)
	code := string(envelope.Error)
	if res.StatusCode < 200 || res.StatusCode >= 300 || (code != "" && code != "null" && code != "false" && code != "0") {
		// Numeric codes are useful; arbitrary server messages may contain secrets.
		var number json.Number
		if json.Unmarshal(envelope.Error, &number) != nil || number.String() == "" {
			var nested struct {
				Code json.Number `json:"code"`
			}
			if json.Unmarshal(envelope.Error, &nested) == nil && nested.Code.String() != "" {
				code = nested.Code.String()
			} else {
				code = "upstream"
			}
		} else {
			code = number.String()
		}
		return &APIError{Status: res.StatusCode, Code: code}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return errors.New("invalid Iframely JSON response")
	}
	return nil
}
