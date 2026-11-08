package linkmedia

import (
	"context"
	"errors"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"ircord/backend/internal/mediafetch"
)

var redditHandler = providerHandler{
	name:         "reddit",
	hosts:        []string{"reddit.com", "www.reddit.com", "old.reddit.com", "redd.it", "www.redd.it"},
	canonicalURL: redditURL,
	connectors:   func(options ResolverOptions) []Connector { return []Connector{options.Reddit, options.Downloader} },
	extractor:    "Reddit",
}

var redditShare = regexp.MustCompile(`^/(?:r/[^/]+/)?s/[A-Za-z0-9]+/?$`)

func redditURL(u *url.URL) (string, error) {
	if _, err := redditID(u.String()); err != nil && !redditShare.MatchString(u.Path) {
		return "", ErrUnsupported
	}
	u.RawQuery = ""
	return u.String(), nil
}

type RedditOptions struct {
	ClientID, ClientSecret, UserAgent string
	HTTPClient                        *http.Client
}

type Reddit struct {
	options RedditOptions
	http    *http.Client
	mu      sync.Mutex
	token   string
	expires time.Time
}

func NewReddit(o RedditOptions) *Reddit {
	if o.UserAgent == "" {
		o.UserAgent = "IRCord/1.0 (link media)"
	}
	h := httpClient(o.HTTPClient)
	// The OAuth secret and bearer token must never follow redirects.
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Reddit{options: o, http: h}
}

var postID = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

func redditID(raw string) (string, error) {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	var id string
	switch strings.ToLower(u.Hostname()) {
	case "redd.it", "www.redd.it":
		if len(parts) == 1 {
			id = parts[0]
		}
	case "reddit.com", "www.reddit.com", "old.reddit.com":
		if len(parts) >= 2 && parts[0] == "comments" {
			id = parts[1]
		}
		if len(parts) >= 4 && parts[0] == "r" && parts[2] == "comments" {
			id = parts[3]
		}
	default:
		return "", ErrUnsupported
	}
	if !postID.MatchString(id) {
		return "", ErrUnsupported
	}
	return id, nil
}

func (r *Reddit) accessToken(ctx context.Context) (string, error) {
	// Serialize token refresh, not media requests. Expiry has a small clock margin.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.token != "" && time.Now().Before(r.expires) {
		return r.token, nil
	}
	if r.options.ClientID == "" || r.options.ClientSecret == "" {
		return "", errors.Join(ErrUnavailable, errors.New("Reddit credentials not configured"))
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.reddit.com/api/v1/access_token", strings.NewReader("grant_type=client_credentials"))
	req.SetBasicAuth(r.options.ClientID, r.options.ClientSecret)
	req.Header.Set("User-Agent", r.options.UserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		Token   string `json:"access_token"`
		Expires int64  `json:"expires_in"`
	}
	if err := request(ctx, r.http, req, "Reddit OAuth", 64<<10, &token); err != nil {
		return "", err
	}
	if token.Token == "" {
		return "", ErrUnavailable
	}
	if token.Expires <= 0 {
		token.Expires = 60
	}
	r.token = token.Token
	r.expires = time.Now().Add(time.Duration(token.Expires)*time.Second - 15*time.Second)
	return r.token, nil
}

type redditPost struct {
	ID, Title, Author, URL, Permalink string
	Selftext                          string `json:"selftext"`
	IsVideo                           bool   `json:"is_video"`
	IsGallery                         bool   `json:"is_gallery"`
	Gallery                           struct {
		Items []struct {
			ID string `json:"media_id"`
		} `json:"items"`
	} `json:"gallery_data"`
	Media map[string]struct {
		Status string `json:"status"`
		MIME   string `json:"m"`
		Source struct {
			URL    string `json:"u"`
			GIF    string `json:"gif"`
			MP4    string `json:"mp4"`
			Width  int    `json:"x"`
			Height int    `json:"y"`
		} `json:"s"`
	} `json:"media_metadata"`
	Preview struct {
		Images []struct {
			Source struct {
				URL    string `json:"url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"source"`
		} `json:"images"`
	} `json:"preview"`
}

func (r *Reddit) Resolve(ctx context.Context, raw string) (Result, error) {
	id, err := redditID(raw)
	if err != nil {
		return Result{}, err
	}
	token, err := r.accessToken(ctx)
	if err != nil {
		return Result{}, err
	}
	load := func(token string) (redditPost, error) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://oauth.reddit.com/comments/"+id+"?raw_json=1&limit=1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", r.options.UserAgent)
		var listings []struct {
			Data struct {
				Children []struct {
					Data redditPost `json:"data"`
				} `json:"children"`
			} `json:"data"`
		}
		if err := request(ctx, r.http, req, "Reddit", 4<<20, &listings); err != nil {
			return redditPost{}, err
		}
		if len(listings) == 0 || len(listings[0].Data.Children) == 0 {
			return redditPost{}, ErrUnavailable
		}
		return listings[0].Data.Children[0].Data, nil
	}
	post, err := load(token)
	var upstream *HTTPError
	if errors.As(err, &upstream) && upstream.Status == http.StatusUnauthorized {
		r.mu.Lock()
		if r.token == token {
			r.token = ""
		}
		r.mu.Unlock()
		token, err = r.accessToken(ctx)
		if err == nil {
			post, err = load(token)
		}
	}
	if err != nil {
		return Result{}, err
	}
	result := Result{URL: "https://www.reddit.com/comments/" + id, Provider: "reddit", ID: id, Title: post.Title, Description: post.Selftext, Author: post.Author}
	if strings.HasPrefix(post.Permalink, "/") && !strings.HasPrefix(post.Permalink, "//") {
		result.URL = "https://www.reddit.com" + post.Permalink
	}
	if post.IsVideo {
		return Result{}, ErrUnavailable
	} // yt-dlp selects a playable combined source
	if post.IsGallery {
		if len(post.Gallery.Items) == 0 || len(post.Gallery.Items) > 30 {
			return Result{}, ErrUnavailable
		}
		for _, item := range post.Gallery.Items {
			media, ok := post.Media[item.ID]
			if !ok || media.Status != "valid" {
				return Result{}, ErrUnavailable
			}
			asset := Asset{Kind: "image", ContentType: media.MIME, Width: media.Source.Width, Height: media.Source.Height}
			asset.URL = html.UnescapeString(media.Source.URL)
			if asset.URL == "" {
				asset.URL = html.UnescapeString(media.Source.GIF)
			}
			if asset.URL == "" && media.Source.MP4 != "" {
				asset.URL = html.UnescapeString(media.Source.MP4)
				asset.Kind = "video"
				asset.ContentType = "video/mp4"
			}
			if !redditMediaURL(asset.URL) {
				return Result{}, ErrUnavailable
			}
			result.Assets = append(result.Assets, asset)
		}
	} else if redditMediaURL(post.URL) {
		result.Assets = []Asset{{URL: post.URL, Kind: "image"}}
	} else if len(post.Preview.Images) > 0 {
		// A link post (an article, say) shows Reddit's preview of what it links to.
		source := post.Preview.Images[0].Source
		if preview := html.UnescapeString(source.URL); redditMediaURL(preview) {
			result.Assets = []Asset{{URL: preview, Kind: "image", Width: source.Width, Height: source.Height}}
		}
	}
	if len(result.Assets) == 0 && (post.Selftext == "" || post.Selftext == "[removed]" || post.Selftext == "[deleted]") {
		return Result{}, ErrUnavailable
	}
	return result, nil
}

func redditMediaURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch u.Hostname() {
	case "i.redd.it", "preview.redd.it", "external-preview.redd.it":
		return true
	}
	return false
}
