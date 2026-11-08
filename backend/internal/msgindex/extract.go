package msgindex

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"ircord/backend/internal/model"
)

// Version names the extraction rules. The backfill of retained history starts
// over whenever it changes, so entries written under older rules are replaced.
const Version = 1

// MaxPerMessage bounds the entries one message adds, like the timeline's
// ten-image gallery.
const MaxPerMessage = 10

// Entry kinds. KindImage and KindGIF are what galleries show; a link catalog
// adds its own kinds to the same index.
const (
	KindImage = "image"
	KindGIF   = "gif"
)

// Entry sources: where in the message the entry came from.
const (
	SourceLink    = "link"    // a direct image link in the text
	SourceUpload  = "upload"  // a file uploaded with the message
	SourcePicker  = "gif"     // a GIF chosen in the GIF picker
	SourcePreview = "preview" // a server link preview: a share page's GIF or an album's images
)

// Entry is one catalogued item of a message, in message order.
type Entry struct {
	URL    string
	Kind   string
	Source string
	// FileID is the stored upload behind an uploaded file's URL, so the entry
	// keeps the file's identity even if its download URL changes.
	FileID string
	Width  int
	Height int
}

// textLink matches links as the frontend renders them (frontend/shared/utils/links.ts).
var textLink = regexp.MustCompile(`https?://[^\s<>"']+`)
var trailing = regexp.MustCompile(`[),.!?]+$`)
var imagePath = regexp.MustCompile(`(?i)\.(png|jpe?g|webp|avif)$`)
var gifPath = regexp.MustCompile(`(?i)\.(gif|apng)$`)
var animatedPath = regexp.MustCompile(`(?i)\.(webp|avif)$`)
var filePath = regexp.MustCompile(`^/files/([A-Za-z0-9_-]{1,128})/[^/]+/`)

// animatedHosts serve animated WebP and AVIF as their GIF formats; gifDomains
// cover every subdomain. Both match the frontend's lists.
var animatedHosts = []string{"media.tenor.com", "media.giphy.com", "i.giphy.com", "static.klipy.com", "cdn.7tv.app"}
var gifDomains = []string{"tenor.com", "giphy.com"}

func animatedHost(host string) bool {
	host = strings.ToLower(host)
	for _, h := range animatedHosts {
		if host == h {
			return true
		}
	}
	for _, d := range gifDomains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// ImageKind classifies a URL by its path as the timeline does: KindGIF for a
// .gif or .apng (or an animated format on a GIF host), KindImage for other
// images, and "" for anything else.
func ImageKind(u *url.URL) string {
	switch {
	case gifPath.MatchString(u.Path), animatedPath.MatchString(u.Path) && animatedHost(u.Hostname()):
		return KindGIF
	case imagePath.MatchString(u.Path):
		return KindImage
	}
	return ""
}

func httpsURL(raw string) *url.URL {
	if len(raw) > 4096 {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil
	}
	return u
}

// Indexed reports whether a message kind can carry catalogued content.
func Indexed(msg model.Message) bool {
	return !msg.Deleted && (msg.Kind == "message" || msg.Kind == "action" || msg.Kind == "notice")
}

// Images lists the images and GIFs a message shows: its upload, its picked GIF,
// direct image links outside code, and the media of its server link preview.
// The same URL counts once. Deleted messages and non-chat lines have none.
func Images(msg model.Message) []Entry {
	if !Indexed(msg) {
		return nil
	}
	out := []Entry{}
	seen := map[string]bool{}
	add := func(e Entry) {
		if len(out) < MaxPerMessage && !seen[e.URL] {
			seen[e.URL] = true
			out = append(out, e)
		}
	}
	upload := ""
	if a := msg.Attachment; a != nil && a.URL != "" {
		ct, ext := a.ContentType, path.Ext(a.Name)
		named := ct == "" && gifPath.MatchString(ext)
		if strings.HasPrefix(ct, "image/") || named || ct == "" && imagePath.MatchString(ext) {
			kind := KindImage
			if ct == "image/gif" || ct == "image/apng" || named {
				kind = KindGIF
			}
			e := Entry{URL: a.URL, Kind: kind, Source: SourceUpload, Width: a.Width, Height: a.Height}
			if u, err := url.Parse(a.URL); err == nil {
				if m := filePath.FindStringSubmatch(u.Path); m != nil {
					e.FileID = m[1]
				}
			}
			upload = a.URL
			add(e)
		}
	}
	if g := msg.GIF; g != nil && httpsURL(g.URL) != nil {
		add(Entry{URL: g.URL, Kind: KindGIF, Source: SourcePicker, Width: g.Width, Height: g.Height})
	}
	if msg.GIF == nil {
		for _, raw := range textLink.FindAllString(OutsideCode(msg.Text), -1) {
			raw = trailing.ReplaceAllString(raw, "")
			if upload != "" && strings.HasSuffix(raw, upload) {
				continue
			}
			u := httpsURL(raw)
			if u == nil {
				continue
			}
			if kind := ImageKind(u); kind != "" {
				add(Entry{URL: raw, Kind: kind, Source: SourceLink})
			}
		}
	}
	if p := msg.Preview; p != nil && msg.PreviewSource == "server" {
		if m := p.Media; m != nil && httpsURL(m.URL) != nil {
			add(Entry{URL: m.URL, Kind: KindGIF, Source: SourcePreview, Width: m.Width, Height: m.Height})
		}
		for _, img := range p.Images {
			if httpsURL(img.URL) != nil {
				add(Entry{URL: img.URL, Kind: KindImage, Source: SourcePreview, Width: img.Width, Height: img.Height})
			}
		}
	}
	return out
}
