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

func widgetCSP(appURL string, document ...string) string {
	ancestors := "'self'"
	if u, err := url.Parse(appURL); err == nil && u.Host != "" && u.User == nil && (u.Scheme == "https" || u.Scheme == "http") && !strings.ContainsAny(u.Host, "'\";\\ \t\r\n") {
		ancestors += " " + u.Scheme + "://" + u.Host
	}
	nonce, images := "'none'", "'none'"
	if len(document) > 0 && len(document[0]) == 48 && strings.Trim(document[0], "0123456789abcdef") == "" { // gitleaks:allow -- hexadecimal validation alphabet
		nonce = "'nonce-" + document[0] + "'"
	}
	if len(document) > 1 {
		images = widgetImageSources(document[1])
	}
	return "default-src 'none'; sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-presentation; script-src " + nonce + "; style-src " + nonce + "; img-src " + images + "; frame-src 'none'; connect-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors " + ancestors
}

func widgetDocument(markup, source, token string, theme ...string) string {
	scheme := "light dark"
	if len(theme) > 0 && (theme[0] == "light" || theme[0] == "dark") {
		scheme = theme[0]
	}
	// The only bridge message is a bounded height. It carries no app authority;
	// the parent validates this frame's source, opaque origin, token and payload.
	encodedToken, _ := json.Marshal(token)
	bridge := fmt.Sprintf(`(()=>{const token=%s;let pending=false;const size=()=>{if(pending)return;pending=true;requestAnimationFrame(()=>{pending=false;parent.postMessage({type:"ircord:widget-size",token,height:Math.max(100,Math.min(800,Math.ceil(document.body.getBoundingClientRect().height)))},"*")})};new ResizeObserver(size).observe(document.body);addEventListener("load",size);size()})()`, encodedToken)
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style nonce="` + html.EscapeString(token) + `">html,body{margin:0;padding:0;overflow-x:hidden;color-scheme:` + scheme + `}body{display:flow-root;font:14px system-ui}img{max-width:100%}a{overflow-wrap:anywhere}</style></head><body>` + markup + `<script nonce="` + html.EscapeString(token) + `">` + bridge + `</script></body></html>`
}
