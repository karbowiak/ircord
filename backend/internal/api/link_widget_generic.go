package api

import (
	"html"
	"net/url"
	"strings"

	parse "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"ircord/backend/internal/mediafetch"
)

// Generic oEmbed markup has no script authority. Static fragments are rebuilt
// from a small element vocabulary; unknown active content becomes a card.
func prepareGenericWidget(markup, source, appURL string, deliveryURL func(string) bool) (string, string, int, float64, bool) {
	if len(markup) == 0 || len(markup) > 128<<10 {
		return "", "", 0, 0, false
	}
	nodes, err := parse.ParseFragment(strings.NewReader(markup), &parse.Node{Type: parse.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return "", "", 0, 0, false
	}
	state := genericWidgetMarkup{source: source, appURL: appURL, deliveryURL: deliveryURL, height: 350}
	for _, node := range nodes {
		if !state.render(node, 0) {
			return "", "", 0, 0, false
		}
	}
	if state.frame != "" {
		// Only a standalone iframe can receive its provider's origin. Wrappers
		// carry no authority, and mixed or script-dependent markup is rejected.
		if state.content || state.frames != 1 {
			return "", "", 0, 0, false
		}
		return "", state.frame, state.height, state.ratio, true
	}
	if !state.content || state.markup.Len() > 256<<10 {
		return "", "", 0, 0, false
	}
	return state.markup.String(), "", state.height, 0, true
}

type genericWidgetMarkup struct {
	markup      strings.Builder
	source      string
	appURL      string
	deliveryURL func(string) bool
	frame       string
	nodes       int
	frames      int
	images      int
	imageBytes  int
	height      int
	ratio       float64
	content     bool
}

func widgetAttribute(node *parse.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Namespace == "" && attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func (s *genericWidgetMarkup) render(node *parse.Node, depth int) bool {
	s.nodes++
	if depth > 32 || s.nodes > 4096 || s.markup.Len() > 256<<10 {
		return false
	}
	switch node.Type {
	case parse.TextNode:
		s.markup.WriteString(html.EscapeString(node.Data))
		s.content = s.content || strings.TrimSpace(node.Data) != ""
		return true
	case parse.CommentNode:
		return true
	case parse.ElementNode:
		if node.Namespace != "" {
			return false
		}
	default:
		return false
	}
	if node.Data == "iframe" {
		if widgetAttribute(node, "srcdoc") != "" {
			return false
		}
		s.frames++
		s.frame = trustedStandaloneWidget(widgetAttribute(node, "src"), s.appURL)
		if s.frame == "" || s.frames != 1 {
			return false
		}
		width, height := widgetPixels(widgetAttribute(node, "width")), widgetPixels(widgetAttribute(node, "height"))
		if height > 0 {
			s.height = max(100, min(height, 800))
			if width > 0 {
				ratio := float64(width) / float64(height)
				if ratio >= 0.05 && ratio <= 20 {
					s.ratio = ratio
				}
			}
		}
		return true
	}
	switch node.Data {
	case "div", "span", "p", "blockquote", "a", "b", "strong", "em", "i", "u", "s", "br", "hr", "pre", "code", "small", "cite", "q", "ul", "ol", "li", "figure", "figcaption", "h1", "h2", "h3", "h4", "h5", "h6", "img":
	default:
		return false
	}
	s.markup.WriteString("<" + node.Data)
	if node.Data == "a" {
		if base, err := url.Parse(s.source); err == nil {
			if link, err := url.Parse(widgetAttribute(node, "href")); err == nil {
				if safe, err := mediafetch.URL(base.ResolveReference(link).String()); err == nil {
					s.markup.WriteString(` href="` + html.EscapeString(safe.String()) + `" target="_blank" rel="noopener noreferrer"`)
				}
			}
		}
	}
	if node.Data == "img" {
		source := widgetAttribute(node, "src")
		s.images++
		s.imageBytes += len(source)
		if s.images > 8 || s.imageBytes > 4096 || !s.deliveryURL(source) || !safeWidgetImageSource(source) {
			return false
		}
		s.markup.WriteString(` src="` + html.EscapeString(source) + `" alt="` + html.EscapeString(widgetAttribute(node, "alt")) + `" referrerpolicy="no-referrer"`)
		s.content = true
	}
	s.markup.WriteString(">")
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if !s.render(child, depth+1) {
			return false
		}
	}
	if node.Data != "img" && node.Data != "br" && node.Data != "hr" {
		s.markup.WriteString("</" + node.Data + ">")
	}
	return true
}

// This is a renderer trust boundary, not a provider extractor. Only an iframe
// already supplied by oEmbed can match these fixed HTTPS player endpoints.
// Everything else must fit the static opaque-document policy above or use a card.
func trustedStandaloneWidget(raw, appURL string) string {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := mediafetch.URL(raw)
	app, _ := url.Parse(appURL)
	if err != nil || u.Scheme != "https" || u.Port() != "" && u.Port() != "443" ||
		app != nil && strings.EqualFold(u.Hostname(), app.Hostname()) || u.RawPath != "" || strings.Contains(u.Path, "\\") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	identifier := func(value string) bool {
		return value != "" && len(value) <= 200 && strings.Trim(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") == ""
	}
	switch u.Hostname() {
	case "www.youtube.com", "www.youtube-nocookie.com":
		if len(parts) == 2 && parts[0] == "embed" && identifier(parts[1]) {
			return u.String()
		}
	case "player.vimeo.com":
		if len(parts) == 2 && parts[0] == "video" && parts[1] != "" && len(parts[1]) <= 20 && strings.Trim(parts[1], "0123456789") == "" {
			return u.String()
		}
	case "open.spotify.com":
		if len(parts) == 3 && parts[0] == "embed" && identifier(parts[2]) {
			switch parts[1] {
			case "track", "album", "playlist", "artist", "episode", "show":
				return u.String()
			}
		}
	}
	return ""
}

// Geometry belongs to our player adapters, never to provider markup or a
// postMessage measurement. The frontend reserves the same box before loading.
func standaloneWidgetDimensions(raw string) (height int, ratio float64) {
	u, err := url.Parse(raw)
	if err == nil && u.Hostname() == "open.spotify.com" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 3 && (parts[1] == "track" || parts[1] == "episode") {
			return 152, 0
		}
		return 352, 0
	}
	return 270, 16.0 / 9.0
}

func safeWidgetImageSource(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil &&
		!strings.ContainsAny(raw, "'\";\\ \t\r\n")
}

func widgetImageSources(markup string) string {
	var sources []string
	seen := map[string]bool{}
	tokens := parse.NewTokenizer(strings.NewReader(markup))
	for {
		switch tokens.Next() {
		case parse.ErrorToken:
			if len(sources) == 0 {
				return "'none'"
			}
			return strings.Join(sources, " ")
		case parse.StartTagToken, parse.SelfClosingTagToken:
			node := tokens.Token()
			if node.Data != "img" {
				continue
			}
			for _, attr := range node.Attr {
				if attr.Key != "src" || !safeWidgetImageSource(attr.Val) || seen[attr.Val] {
					continue
				}
				seen[attr.Val] = true
				sources = append(sources, attr.Val)
			}
		}
	}
}
