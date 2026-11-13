package linkmedia

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"ircord/backend/internal/mediafetch"
)

var instagramHandler = providerHandler{
	name:          "instagram",
	hosts:         []string{"instagram.com", "www.instagram.com"},
	canonicalURL:  func(u *url.URL) (string, error) { canonical, _, err := instagramURL(u.String()); return canonical, err },
	extractor:     "Instagram",
	photoFallback: ytInfo.instagramPhoto,
	finishExtract: finishInstagramExtract,
	sourceExpiry:  instagramSourceExpiry,
}

func instagramSourceExpiry(source *url.URL, values url.Values) time.Time {
	if strings.HasSuffix(source.Hostname(), ".cdninstagram.com") || strings.HasSuffix(source.Hostname(), ".fbcdn.net") {
		if number, err := strconv.ParseInt(values.Get("oe"), 16, 64); err == nil && number > 0 {
			return time.Unix(number, 0)
		}
	}
	return time.Time{}
}

func finishInstagramExtract(result *Result) {
	if len(result.Assets) == 1 && result.Assets[0].Kind == "image" && strings.HasPrefix(result.Title, "Video by ") {
		result.Title = "Photo by " + strings.TrimPrefix(result.Title, "Video by ")
	}
}

type Instagram struct{ http *http.Client }

func NewInstagram(client *http.Client) *Instagram {
	if client == nil {
		client = nativeHTTPClient()
	}
	return &Instagram{http: httpClient(client)}
}

var instagramPath = regexp.MustCompile(`^/(?:p|reel|reels|tv)/([A-Za-z0-9_-]+)/?$`)

func instagramURL(raw string) (string, string, error) {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return "", "", err
	}
	if u.Scheme != "https" || u.RawPath != "" || u.Port() != "" && u.Port() != "443" || !socialQuery(u) || u.Hostname() != "www.instagram.com" && u.Hostname() != "instagram.com" {
		return "", "", ErrUnsupported
	}
	match := instagramPath.FindStringSubmatch(u.Path)
	if match == nil {
		return "", "", ErrUnsupported
	}
	if len(match[1]) > 28 {
		return "", "", ErrRestricted
	}
	u.Host = "www.instagram.com"
	u.Path = "/p/" + match[1] + "/"
	u.RawQuery = ""
	return u.String(), match[1], nil
}

// Resolve reads only the public same-post embed's structured data, never an OG
// thumbnail or account page. It is the fallback for missing yt-dlp photo data.
func (i *Instagram) Resolve(ctx context.Context, raw string) (Result, error) {
	canonical, id, err := instagramURL(raw)
	if err != nil {
		return Result{}, err
	}
	embedURL := canonical + "embed/captioned/"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, embedURL, nil)
	req.Header.Set("User-Agent", "IRCord-LinkPreview/1.0")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", "text/html, application/json;q=0.9")
	res, err := i.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, ErrUnavailable
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		res.Body.Close()
		return Result{}, ErrRestricted
	}
	if res.Request == nil || res.Request.URL == nil || !sameInstagramEmbed(res.Request.URL, id) || res.Header.Get("Content-Encoding") != "" && res.Header.Get("Content-Encoding") != "identity" {
		res.Body.Close()
		return Result{}, ErrRestricted
	}
	data, err := readResponse(res, "Instagram", 1<<20)
	if err != nil {
		return Result{}, err
	}
	var documents [][]byte
	if json.Valid(data) {
		documents = append(documents, data)
	} else {
		tokens := html.NewTokenizer(bytes.NewReader(data))
		tokens.SetMaxBuf(1 << 20)
		inScript := false
		for {
			switch tokens.Next() {
			case html.ErrorToken:
				if tokens.Err() != io.EOF {
					return Result{}, ErrUnavailable
				}
				goto parsed
			case html.StartTagToken:
				name, _ := tokens.TagName()
				inScript = string(name) == "script"
			case html.EndTagToken:
				inScript = false
			case html.TextToken:
				if !inScript {
					continue
				}
				text := strings.TrimSpace(string(tokens.Text()))
				text = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(text, "window._sharedData =")), ";")
				if json.Valid([]byte(text)) {
					documents = append(documents, []byte(text))
				} else if strings.HasPrefix(text, "window.__additionalDataLoaded(") {
					// Decode only the JSON argument of the known public embed wrapper;
					// no script is evaluated and no callback/module is loaded.
					if start := strings.IndexByte(text, '{'); start >= 0 {
						var value json.RawMessage
						if json.NewDecoder(strings.NewReader(text[start:])).Decode(&value) == nil {
							documents = append(documents, value)
						}
					}
				}
			}
		}
	}
parsed:
	for _, document := range documents {
		var root any
		if json.Unmarshal(document, &root) != nil {
			continue
		}
		if node := instagramNode(root, id, 0); node != nil {
			result, err := instagramResult(node)
			if err != nil {
				return Result{}, err
			}
			result.URL, result.ID, result.Provider = canonical, id, "instagram"
			if err := validateSocialResult(result, "instagram", id); err != nil {
				return Result{}, err
			}
			return result, nil
		}
	}
	return Result{}, ErrUnavailable
}

func sameInstagramEmbed(u *url.URL, id string) bool {
	if u.Scheme != "https" || u.Host != "www.instagram.com" || u.User != nil || u.RawPath != "" || u.RawQuery != "" {
		return false
	}
	return u.Path == "/p/"+id+"/embed/captioned/" || u.Path == "/p/"+id+"/embed/"
}

func instagramNode(value any, id string, depth int) map[string]any {
	if depth > 32 {
		return nil
	}
	switch v := value.(type) {
	case map[string]any:
		kind := stringValue(v, "__typename")
		code := stringValue(v, "shortcode")
		if code == "" {
			code = stringValue(v, "code")
		}
		if code == id && (kind == "GraphImage" || kind == "GraphVideo" || kind == "GraphSidecar" || kind == "XDTGraphImage" || kind == "XDTGraphVideo" || kind == "XDTGraphSidecar" || number(v, "media_type") > 0) {
			return v
		}
		for _, child := range v {
			if node := instagramNode(child, id, depth+1); node != nil {
				return node
			}
		}
	case []any:
		for _, child := range v {
			if node := instagramNode(child, id, depth+1); node != nil {
				return node
			}
		}
	}
	return nil
}

func instagramResult(node map[string]any) (Result, error) {
	result := Result{Description: stringValue(node, "accessibility_caption")}
	if owner, ok := node["owner"].(map[string]any); ok {
		result.Author = stringValue(owner, "username")
	}
	if user, ok := node["user"].(map[string]any); ok {
		result.Author = stringValue(user, "username")
	}
	if caption, ok := node["caption"].(map[string]any); ok {
		result.Description = stringValue(caption, "text")
	}
	if caption, ok := node["edge_media_to_caption"].(map[string]any); ok {
		if edges, ok := caption["edges"].([]any); ok && len(edges) > 0 {
			if edge, ok := edges[0].(map[string]any); ok {
				if n, ok := edge["node"].(map[string]any); ok {
					result.Description = stringValue(n, "text")
				}
			}
		}
	}
	nodes := []any{node}
	isSidecar := strings.HasSuffix(stringValue(node, "__typename"), "GraphSidecar") || number(node, "media_type") == 8
	if carousel, ok := node["carousel_media"].([]any); ok {
		nodes = carousel
	}
	if sidecar, ok := node["edge_sidecar_to_children"].(map[string]any); ok {
		edges, _ := sidecar["edges"].([]any)
		nodes = nil
		for _, item := range edges {
			edge, ok := item.(map[string]any)
			if !ok {
				return Result{}, ErrUnavailable
			}
			nodes = append(nodes, edge["node"])
		}
		if count := number(sidecar, "count"); count > 0 && int(count) != len(nodes) {
			return Result{}, ErrUnavailable
		}
	} else if isSidecar {
		if _, ok := node["carousel_media"].([]any); !ok {
			return Result{}, ErrUnavailable
		}
	}
	if count := number(node, "carousel_media_count"); count > 0 && int(count) != len(nodes) {
		return Result{}, ErrUnavailable
	}
	if len(nodes) > maxNativeAssets {
		return Result{}, ErrLimit
	}
	if len(nodes) == 0 {
		return Result{}, ErrUnavailable
	}
	for _, value := range nodes {
		n, ok := value.(map[string]any)
		if !ok {
			return Result{}, ErrUnavailable
		}
		asset := Asset{Kind: "image", ContentType: "image/jpeg", URL: stringValue(n, "display_url"), Headers: http.Header{"Referer": {"https://www.instagram.com/"}}}
		if dim, ok := n["dimensions"].(map[string]any); ok {
			asset.Width = int(number(dim, "width"))
			asset.Height = int(number(dim, "height"))
		}
		isVideo, _ := n["is_video"].(bool)
		if isVideo || number(n, "media_type") == 2 || strings.HasSuffix(stringValue(n, "__typename"), "GraphVideo") {
			asset.Kind, asset.ContentType, asset.URL = "video", "video/mp4", stringValue(n, "video_url")
			asset.Duration = number(n, "video_duration")
			if versions, ok := n["video_versions"].([]any); ok {
				asset.URL, asset.Width, asset.Height = bestInstagramSource(versions)
			}
		} else if versions, ok := n["image_versions2"].(map[string]any); ok {
			if candidates, ok := versions["candidates"].([]any); ok {
				asset.URL, asset.Width, asset.Height = bestInstagramSource(candidates)
				var images []ytImage
				for _, value := range candidates {
					if candidate, ok := value.(map[string]any); ok {
						images = append(images, ytImage{URL: stringValue(candidate, "url"), Width: int(number(candidate, "width")), Height: int(number(candidate, "height"))})
					}
				}
				asset = instagramPhotoRenditions(asset, images)
			}
		}
		if _, err := mediafetch.URL(asset.URL); err != nil {
			return Result{}, ErrUnavailable
		}
		result.Assets = append(result.Assets, asset)
	}
	return result, nil
}

func bestInstagramSource(candidates []any) (raw string, width, height int) {
	for _, value := range candidates {
		if item, ok := value.(map[string]any); ok {
			w, h := int(number(item, "width")), int(number(item, "height"))
			if source := stringValue(item, "url"); source != "" && (raw == "" || int64(w)*int64(h) > int64(width)*int64(height)) {
				raw, width, height = source, w, h
			}
		}
	}
	return
}

func stringValue(node map[string]any, key string) string {
	value, _ := node[key].(string)
	return value
}
func number(node map[string]any, key string) float64 { value, _ := node[key].(float64); return value }

// Instagram's extractor exposes each photo's image_versions2 candidates as
// thumbnails, with an explicitly empty formats list. These are the original
// photo variants, not the post's single Open Graph teaser. Keep this exception
// provider-specific and never turn an unavailable/DRM video into a still image.
func (i ytInfo) instagramPhoto() (Asset, error) {
	if !strings.EqualFold(i.ExtractorKey, "Instagram") || i.Formats == nil || len(i.Formats) != 0 || i.Duration != nil || i.HasDRM {
		return Asset{}, ErrUnavailable
	}
	var best ytImage
	for _, candidate := range i.Thumbnails {
		u, err := mediafetch.URL(candidate.URL)
		if err != nil || u.Scheme != "https" {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host != "cdninstagram.com" && !strings.HasSuffix(host, ".cdninstagram.com") && host != "fbcdn.net" && !strings.HasSuffix(host, ".fbcdn.net") {
			continue
		}
		// yt-dlp sorts image variants best-last. Some current Instagram photo
		// responses omit all dimensions, while video poster sizes are populated.
		if best.URL == "" || candidate.URL == i.Thumbnail || best.URL != i.Thumbnail && int64(candidate.Width)*int64(candidate.Height) >= int64(best.Width)*int64(best.Height) {
			best = candidate
		}
	}
	if best.URL == "" {
		return Asset{}, ErrUnavailable
	}
	asset := Asset{URL: best.URL, Kind: "image", Width: best.Width, Height: best.Height, Headers: http.Header{}}
	for key, value := range i.Headers {
		copyMediaHeader(asset.Headers, key, value)
	}
	return instagramPhotoRenditions(asset, i.Thumbnails), nil
}

// yt-dlp's photo variants can omit dimensions, so final selection must inspect
// bytes. Keep a bounded best-last retry list, never a single post teaser.
func instagramPhotoRenditions(asset Asset, candidates []ytImage) Asset {
	const maxRenditions = 16
	seen := map[string]bool{asset.URL: true}
	for index := len(candidates) - 1; index >= 0 && len(asset.imageRenditions) < maxRenditions-1; index-- {
		candidate := candidates[index]
		if seen[candidate.URL] || !socialMediaURL(candidate.URL, "instagram") {
			continue
		}
		seen[candidate.URL] = true
		asset.imageRenditions = append(asset.imageRenditions, candidate.URL)
	}
	return asset
}
