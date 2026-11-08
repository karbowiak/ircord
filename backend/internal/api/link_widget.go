package api

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"slices"
	"strconv"
	"strings"

	parse "golang.org/x/net/html"
	"ircord/backend/internal/iframely"
	"ircord/backend/internal/mediafetch"
)

// The self-hosted Iframely engine returns metadata without HTML for ordinary
// pages. Render only that returned data in the same isolated widget document;
// this does not fetch the page again or turn its links into native media.
func summaryWidget(data iframely.Response, source string) string {
	if data.Meta.Title == "" && data.Meta.Description == "" {
		return ""
	}
	u, err := mediafetch.URL(source)
	if err != nil {
		return ""
	}
	title := data.Meta.Title
	if title == "" {
		title = u.Hostname()
	}
	site := data.Meta.Site
	if site == "" {
		site = u.Hostname()
	}
	picture, compact := widgetImage(data)
	class := "summary"
	if compact {
		class += " is-compact"
	}
	if markup := providerSummaryWidget(u, title, data.Meta.Description, picture); markup != "" {
		return markup
	}
	markup := `<style>.summary{border:1px solid color-mix(in srgb,CanvasText 16%,transparent);border-radius:12px;overflow:hidden;background:Canvas;color:CanvasText}.summary a{display:block;color:inherit;text-decoration:none}.summary a:focus-visible{outline:2px solid Highlight;outline-offset:-3px}.summary img{display:block;width:100%;max-height:240px;object-fit:contain}.summary div{padding:12px}.summary h2{font-size:15px;line-height:1.5;margin:0 0 6px;overflow-wrap:anywhere}.summary p{font-size:13px;line-height:1.6;margin:0 0 8px;overflow-wrap:anywhere}.summary small{font-size:12px}.summary.is-compact a{display:grid;grid-template-columns:minmax(0,1fr) 64px;gap:12px;align-items:start;padding:12px}.summary.is-compact div{padding:0;grid-column:1;grid-row:1;min-width:0}.summary.is-compact img{width:64px;height:64px;grid-column:2;grid-row:1}</style><article class="` + class + `"><a href="` + html.EscapeString(u.String()) + `" target="_blank" rel="noopener noreferrer nofollow">`
	if picture != "" {
		markup += `<img src="` + html.EscapeString(picture) + `" alt="" loading="lazy" referrerpolicy="no-referrer" onerror="this.remove()">`
	}
	return markup + `<div><h2>` + html.EscapeString(title) + `</h2><p>` + html.EscapeString(data.Meta.Description) + `</p><small>` + html.EscapeString(site) + `</small></div></a></article>`
}

// summaryCard is the page summary for the client to draw: title, description, site, preview
// image (with its size when Iframely knows it) and site icon.
func summaryCard(data iframely.Response, source string) *LinkMediaCard {
	if data.Meta.Title == "" && data.Meta.Description == "" {
		return nil
	}
	u, err := mediafetch.URL(source)
	if err != nil {
		return nil
	}
	card := &LinkMediaCard{Site: data.Meta.Site, Title: data.Meta.Title, Description: data.Meta.Description}
	if card.Site == "" {
		card.Site = strings.TrimPrefix(u.Hostname(), "www.")
	}
	if card.Title == "" {
		card.Title = card.Site
	}
	card.Image, card.ImageWidth, card.ImageHeight = linkImage(data, "thumbnail", "image")
	card.Icon, _, _ = linkImage(data, "icon")
	return card
}

// linkImage is the first HTTPS image link carrying one of rels, with its size when known.
func linkImage(data iframely.Response, rels ...string) (string, int, int) {
	groups := make([]string, 0, len(data.Links))
	for key := range data.Links {
		groups = append(groups, key)
	}
	slices.Sort(groups)
	for _, key := range groups {
		for _, link := range data.Links[key] {
			if !slices.ContainsFunc(rels, func(rel string) bool { return slices.Contains(link.Rel, rel) }) {
				continue
			}
			if link.Type != "" && !strings.HasPrefix(link.Type, "image") {
				continue
			}
			imageURL, err := mediafetch.URL(link.Href)
			if err != nil || imageURL.Scheme != "https" {
				continue
			}
			var width, height int
			_ = json.Unmarshal(link.Media["width"], &width)
			_ = json.Unmarshal(link.Media["height"], &height)
			return imageURL.String(), width, height
		}
	}
	return "", 0, 0
}

func widgetImage(data iframely.Response) (string, bool) {
	var picture string
	compact := false
	groups := make([]string, 0, len(data.Links))
	for key := range data.Links {
		groups = append(groups, key)
	}
	slices.Sort(groups)
	for _, key := range groups {
		for _, link := range data.Links[key] {
			if !slices.Contains(link.Rel, "thumbnail") && !slices.Contains(link.Rel, "image") {
				continue
			}
			// Iframely also uses the generic "image" type when the publisher
			// supplies a thumbnail without a specific MIME type.
			if !slices.Contains([]string{"image", "image/jpeg", "image/jpg", "image/png", "image/webp", "image/avif", "image/gif"}, link.Type) {
				continue
			}
			imageURL, err := mediafetch.URL(link.Href)
			if err == nil && imageURL.Scheme == "https" {
				picture = imageURL.String()
				var width, height int
				_ = json.Unmarshal(link.Media["width"], &width)
				_ = json.Unmarshal(link.Media["height"], &height)
				compact = width > 0 && height > 0 && width <= 160 && height <= 160
				break
			}
		}
		if picture != "" {
			break
		}
	}
	return picture, compact
}

// A publisher's standalone iframe can use its own origin/storage safely. Raw
// HTML and script-based widgets instead live in our opaque-origin document.
func externalWidget(markup, appURL string) (string, int) {
	frame, height, _ := externalWidgetLayout(markup, appURL)
	return frame, height
}

// Known script widgets ultimately create a provider iframe. Use that same
// frame directly: inheriting our opaque wrapper's sandbox breaks their loaders.
// Arbitrary publisher scripts still stay in the isolated widget document.
func providerWidgetLayout(markup, source, appURL string) (string, int, float64) {
	if frame := scriptWidgetFrame(markup, source); frame != "" {
		markup = `<iframe src="` + html.EscapeString(frame) + `" height="600"></iframe>`
	}
	return externalWidgetLayout(markup, appURL)
}

// Only the provider URL and numeric sizing cross into the app. Publisher
// players keep their own origin; arbitrary HTML stays in our opaque document.
func externalWidgetLayout(markup, appURL string) (string, int, float64) {
	tokens := parse.NewTokenizer(strings.NewReader(markup))
	var frames []string
	script := false
	height := 350
	ratio := 0.0
	for {
		kind := tokens.Next()
		if kind == parse.ErrorToken {
			break
		}
		if kind != parse.StartTagToken && kind != parse.SelfClosingTagToken {
			continue
		}
		tag := tokens.Token()
		if tag.Data == "script" {
			script = true
		}
		if tag.Data != "iframe" {
			for _, attr := range tag.Attr {
				if attr.Key != "style" {
					continue
				}
				for _, declaration := range strings.Split(attr.Val, ";") {
					property, value, ok := strings.Cut(declaration, ":")
					property = strings.TrimSpace(strings.ToLower(property))
					value = strings.TrimSpace(value)
					if ok && (property == "padding-bottom" || property == "padding-top") && strings.HasSuffix(value, "%") {
						if percent, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64); err == nil && percent >= 5 && percent <= 2000 {
							ratio = 100 / percent
						}
					}
				}
			}
			continue
		}
		var source string
		attributeHeight, styleHeight, attributeWidth := 0, 0, 0
		for _, attr := range tag.Attr {
			if attr.Key == "src" {
				source = attr.Val
			}
			if attr.Key == "height" {
				attributeHeight = widgetPixels(attr.Val)
			}
			if attr.Key == "width" {
				attributeWidth = widgetPixels(attr.Val)
			}
			if attr.Key == "style" {
				for _, declaration := range strings.Split(attr.Val, ";") {
					property, value, ok := strings.Cut(declaration, ":")
					if ok && strings.EqualFold(strings.TrimSpace(property), "height") {
						styleHeight = widgetPixelHeight(value)
					}
				}
			}
		}
		if styleHeight > 0 {
			height = styleHeight
		} else if attributeHeight > 0 {
			height = max(100, min(attributeHeight, 800))
			if ratio == 0 && attributeWidth > 0 {
				ratio = float64(attributeWidth) / float64(attributeHeight)
			}
		}
		frames = append(frames, source)
	}
	if script || len(frames) != 1 {
		return "", height, 0
	}
	raw := frames[0]
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := mediafetch.URL(raw)
	app, _ := url.Parse(appURL)
	if err != nil || u.Scheme != "https" || app != nil && strings.EqualFold(u.Hostname(), app.Hostname()) {
		return "", height, 0
	}
	return u.String(), height, ratio
}

func widgetPixelHeight(raw string) int {
	if n := widgetPixels(raw); n > 0 {
		return max(100, min(n, 800))
	}
	return 0
}

func widgetPixels(raw string) int {
	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "!important"))
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(raw, "px")))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func widgetCSP(appURL string) string {
	ancestors := "'self'"
	if u, err := url.Parse(appURL); err == nil && u.Host != "" {
		ancestors += " " + u.Scheme + "://" + u.Host
	}
	return "default-src 'none'; sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-presentation; script-src https: 'unsafe-inline'; style-src https: 'unsafe-inline'; img-src https: data:; media-src https:; frame-src https:; connect-src https:; font-src https: data:; base-uri https:; form-action 'none'; frame-ancestors " + ancestors
}

func widgetDocument(markup, source, token string, theme ...string) string {
	scheme := "light dark"
	if len(theme) > 0 && (theme[0] == "light" || theme[0] == "dark") {
		scheme = theme[0]
	}
	// The only bridge message is a bounded height. It carries no app authority;
	// the parent validates this frame's source, opaque origin, token and payload.
	bridge := fmt.Sprintf(`(()=>{const token=%q;let pending=false;const size=()=>{if(pending)return;pending=true;requestAnimationFrame(()=>{pending=false;parent.postMessage({type:"ircord:widget-size",token,height:Math.max(100,Math.min(800,Math.ceil(document.body.getBoundingClientRect().height)))},"*")})};new ResizeObserver(size).observe(document.body);addEventListener("load",size);size()})()`, token)
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><base href="` + html.EscapeString(source) + `" target="_blank"><style>html,body{margin:0;padding:0;overflow-x:hidden;color-scheme:` + scheme + `}body{display:flow-root;font:14px system-ui}img,video,iframe{max-width:100%}iframe{border:0}a{overflow-wrap:anywhere}</style></head><body>` + markup + `<script>` + bridge + `</script></body></html>`
}
