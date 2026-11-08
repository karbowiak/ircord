package preview

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"ircord/backend/internal/model"
)

// oEmbed (https://oembed.com) lets a page advertise a small metadata document through
// `<link rel="alternate" type="application/json+oembed" href="…">` or its text/xml+oembed
// twin. Any page that advertises one gets it read, without a provider list: the document's
// title, author, provider and thumbnail fill the ordinary card, and Open Graph fills whatever
// it leaves out. The document's `html` snippet is never read, so no provider markup or player
// reaches clients; players stay limited to the typed embeds (YouTube) the client builds itself.

// oembedLink is an oEmbed document a page advertises.
type oembedLink struct {
	href string
	xml  bool
}

// oembedType reads a `<link>` type attribute: JSON, XML, or not oEmbed at all.
func oembedType(kind string) (isXML, ok bool) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "application/json+oembed":
		return false, true
	case "text/xml+oembed", "application/xml+oembed":
		return true, true
	}
	return false, false
}

// oembedDocument holds the fields of an oEmbed response a card uses. Numbers are read as
// strings because providers send widths both as JSON numbers and as quoted strings.
type oembedDocument struct {
	Type            string     `json:"type" xml:"type"`
	Title           string     `json:"title" xml:"title"`
	AuthorName      string     `json:"author_name" xml:"author_name"`
	ProviderName    string     `json:"provider_name" xml:"provider_name"`
	URL             string     `json:"url" xml:"url"`
	Width           flexNumber `json:"width" xml:"width"`
	Height          flexNumber `json:"height" xml:"height"`
	ThumbnailURL    string     `json:"thumbnail_url" xml:"thumbnail_url"`
	ThumbnailWidth  flexNumber `json:"thumbnail_width" xml:"thumbnail_width"`
	ThumbnailHeight flexNumber `json:"thumbnail_height" xml:"thumbnail_height"`
}

// flexNumber accepts 480, "480" or null.
type flexNumber string

func (n *flexNumber) UnmarshalJSON(data []byte) error {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "null" {
		text = ""
	}
	*n = flexNumber(text)
	return nil
}
func (n flexNumber) int() int { return dimension(string(n)) }

// decodeOEmbed reads a bounded oEmbed response in the advertised format.
func decodeOEmbed(body io.Reader, isXML bool) (oembedDocument, error) {
	var document oembedDocument
	limited := io.LimitReader(body, maxOEmbed)
	var err error
	if isXML {
		decoder := xml.NewDecoder(limited)
		// encoding/xml never resolves DTDs or external entities.
		var root struct {
			XMLName xml.Name `xml:"oembed"`
			oembedDocument
		}
		err = decoder.Decode(&root)
		document = root.oembedDocument
	} else {
		err = json.NewDecoder(limited).Decode(&document)
	}
	if err != nil {
		return oembedDocument{}, errors.New("oEmbed response is unreadable")
	}
	return document, nil
}

// oembedBudget bounds the oEmbed lookup within a page fetch, so a slow provider leaves the
// page's own deadline enough time to check its images.
var oembedBudget = 3 * time.Second

// fetchOEmbed fetches an advertised oEmbed document under the page's own rules: public HTTPS
// destinations only (checked before every request and again by the pinned dialer), at most
// three validated redirects, no credentials and a bounded body, within its own short budget.
func (s *Service) fetchOEmbed(ctx context.Context, base *url.URL, link oembedLink) (oembedDocument, error) {
	ctx, cancel := context.WithTimeout(ctx, oembedBudget)
	defer cancel()
	reference, err := url.Parse(strings.TrimSpace(link.href))
	if err != nil {
		return oembedDocument{}, errors.New("invalid oEmbed address")
	}
	u, err := s.validate(ctx, base.ResolveReference(reference).String())
	if err != nil {
		return oembedDocument{}, err
	}
	accept := "application/json"
	if link.xml {
		accept = "text/xml, application/xml"
	}
	response, err := s.get(ctx, u, accept)
	if err != nil {
		return oembedDocument{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return oembedDocument{}, fmt.Errorf("oEmbed returned HTTP %d", response.StatusCode)
	}
	return decodeOEmbed(response.Body, link.xml)
}

// get requests a validated URL without credentials, following at most three redirects, each
// validated like the first address. The response is never a redirect.
func (s *Service) get(ctx context.Context, u *url.URL, accept string) (*http.Response, error) {
	for redirects := 0; redirects <= 3; redirects++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		req.Header.Set("Accept", accept)
		req.Header.Set("User-Agent", "IRCord-LinkPreview/1")
		response, err := s.client.Do(req)
		if err != nil {
			return nil, errors.New("preview request failed")
		}
		if response.StatusCode < 300 || response.StatusCode >= 400 {
			return response, nil
		}
		location, err := response.Location()
		response.Body.Close()
		if err != nil {
			return nil, errors.New("invalid preview redirect")
		}
		if u, err = s.validate(ctx, location.String()); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("too many preview redirects")
}

// applyOEmbed fills the card's text from an oEmbed document; fields it leaves empty keep their
// Open Graph values. It returns the document's image (for photos, the picture itself) with its
// declared size; the caller swaps it in only once it passes the image checks, so a refused
// thumbnail leaves the Open Graph image in place.
func applyOEmbed(value *model.LinkPreview, base *url.URL, document oembedDocument) (image string, width, height int) {
	set := func(target *string, text string, limit int) {
		if text = bounded(text, limit); text != "" {
			*target = text
		}
	}
	set(&value.Title, document.Title, 200)
	set(&value.Author, document.AuthorName, 100)
	set(&value.SiteName, document.ProviderName, 100)
	kind := strings.ToLower(strings.TrimSpace(document.Type))
	if kind == "video" && value.Kind == "" {
		value.Kind = "video"
	}
	candidate, width, height := document.ThumbnailURL, document.ThumbnailWidth.int(), document.ThumbnailHeight.int()
	if kind == "photo" && document.URL != "" {
		candidate, width, height = document.URL, document.Width.int(), document.Height.int()
	}
	reference, err := url.Parse(strings.TrimSpace(candidate))
	if err != nil || candidate == "" {
		return "", 0, 0
	}
	if resolved := base.ResolveReference(reference); resolved.Scheme == "https" {
		return resolved.String(), width, height
	}
	return "", 0, 0
}

// oembedTag reads `<link rel="alternate" type="…+oembed" href="…">`.
func oembedTag(token html.Token) *oembedLink {
	var rel, kind, href string
	for _, a := range token.Attr {
		switch a.Key {
		case "rel":
			rel = strings.ToLower(a.Val)
		case "type":
			kind = a.Val
		case "href":
			href = strings.TrimSpace(a.Val)
		}
	}
	isXML, ok := oembedType(kind)
	if !ok || href == "" || !contains(strings.Fields(rel), "alternate") {
		return nil
	}
	return &oembedLink{href: href, xml: isXML}
}
