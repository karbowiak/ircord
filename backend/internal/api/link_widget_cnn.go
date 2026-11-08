package api

import (
	"net/url"
	"strings"

	"ircord/backend/internal/iframely"
)

func cnnWidgetOptions(widget *LinkMediaWidget, data iframely.Response) {
	provider, err := url.Parse(widget.URL)
	if err == nil && strings.EqualFold(provider.Hostname(), "fave.api.cnn.io") {
		widget.ClickToLoad = true
		widget.Thumbnail, _ = widgetImage(data)
	}
}
