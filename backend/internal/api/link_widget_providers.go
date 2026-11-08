package api

import (
	"net/url"
	"slices"
	"strings"

	parse "golang.org/x/net/html"
	"ircord/backend/internal/iframely"
	"ircord/backend/internal/mediafetch"
)

// Widget-only providers remain Iframely-backed. Register a presentation adapter
// here when a publisher needs a specific summary, script frame, or player option.
var widgetFrameRenderers = []func(*url.URL, scriptWidgetMarkup) string{
	xWidgetFrame,
	blueskyWidgetFrame,
}
var widgetOptionHandlers = []func(*LinkMediaWidget, iframely.Response){
	cnnWidgetOptions,
}

func applyWidgetOptions(widget *LinkMediaWidget, data iframely.Response) {
	for _, configure := range widgetOptionHandlers {
		configure(widget, data)
	}
}

type scriptWidgetMarkup struct {
	blockquotes []map[string]string
	scripts     []*url.URL
}

func (m scriptWidgetMarkup) hasScript(path string, hosts ...string) bool {
	for _, script := range m.scripts {
		if script.Path == path && slices.Contains(hosts, script.Hostname()) {
			return true
		}
	}
	return false
}

func scriptWidgetFrame(markup, source string) string {
	u, err := mediafetch.URL(source)
	if err != nil {
		return ""
	}
	data := inspectScriptWidget(markup)
	for _, render := range widgetFrameRenderers {
		if frame := render(u, data); frame != "" {
			return frame
		}
	}
	return ""
}

func inspectScriptWidget(markup string) scriptWidgetMarkup {
	var out scriptWidgetMarkup
	tokens := parse.NewTokenizer(strings.NewReader(markup))
	for {
		kind := tokens.Next()
		if kind == parse.ErrorToken {
			return out
		}
		if kind != parse.StartTagToken && kind != parse.SelfClosingTagToken {
			continue
		}
		tag := tokens.Token()
		if tag.Data == "blockquote" {
			attributes := map[string]string{}
			for _, attr := range tag.Attr {
				attributes[attr.Key] = attr.Val
			}
			out.blockquotes = append(out.blockquotes, attributes)
		}
		if tag.Data == "script" {
			for _, attr := range tag.Attr {
				if attr.Key != "src" {
					continue
				}
				script, err := url.Parse(attr.Val)
				if err == nil && script.Scheme == "https" && script.User == nil && script.Port() == "" {
					out.scripts = append(out.scripts, script)
				}
			}
		}
	}
}
