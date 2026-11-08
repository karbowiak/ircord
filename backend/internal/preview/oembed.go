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

// fetchOEmbed fetches an advertised oEmbed document under the page's own rules: public HTTPS
// destinations only (checked before every request and again by the pinned dialer), at most
// three redirects, no credentials, a bounded body, and the caller's deadline.
func (s *Service) fetchOEmbed(ctx context.Context, base *url.URL, link oembedLink) (oembedDocument, error) {
	reference, err := url.Parse(strings.TrimSpace(link.href))
	if err != nil {
		return oembedDocument{}, errors.New("invalid oEmbed address")
	}
	u, err := s.validate(ctx, base.ResolveReference(reference).String())
	if err != nil {
		return oembedDocument{}, err
	}
	for redirects := 0; redirects <= 3; redirects++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if link.xml {
			req.Header.Set("Accept", "text/xml, application/xml")
		} else {
			req.Header.Set("Accept", "application/json")
		}
		req.Header.Set("User-Agent", "IRCord-LinkPreview/1")
		response, err := s.client.Do(req)
		if err != nil {
			return oembedDocument{}, errors.New("oEmbed request failed")
		}
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			location, err := response.Location()
			response.Body.Close()
			if err != nil {
				return oembedDocument{}, errors.New("invalid oEmbed redirect")
			}
			if u, err = s.validate(ctx, location.String()); err != nil {
				return oembedDocument{}, err
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return oembedDocument{}, fmt.Errorf("oEmbed returned HTTP %d", response.StatusCode)
		}
		document, err := decodeOEmbed(response.Body, link.xml)
		response.Body.Close()
		return document, err
	}
	return oembedDocument{}, errors.New("too many oEmbed redirects")
}

// applyOEmbed fills the card from an oEmbed document; fields it leaves empty keep their Open
// Graph values. A new image replaces the Open Graph one together with its declared size.
func applyOEmbed(value *model.LinkPreview, base *url.URL, document oembedDocument) {
	set := func(target *string, text string, limit int) {
		if text = bounded(text, limit); text != "" {
			*target = text
		}
	}
	set(&value.Title, document.Title, 200)
	set(&value.Author, document.AuthorName, 100)
	set(&value.SiteName, document.ProviderName, 100)
	kind := strings.ToLower(strings.TrimSpace(document.Type))
	image, width, height := document.ThumbnailURL, document.ThumbnailWidth.int(), document.ThumbnailHeight.int()
	if kind == "photo" && document.URL != "" {
		// A photo document's url is the picture itself.
		image, width, height = document.URL, document.Width.int(), document.Height.int()
	}
	if reference, err := url.Parse(strings.TrimSpace(image)); err == nil && image != "" {
		if resolved := base.ResolveReference(reference); resolved.Scheme == "https" {
			value.Image, value.ImageWidth, value.ImageHeight = resolved.String(), width, height
		}
	}
	if kind == "video" && value.Kind == "" {
		value.Kind = "video"
	}
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
