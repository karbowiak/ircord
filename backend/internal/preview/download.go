package preview

import (
	"bytes"
	"context"
	"errors"
	stdimage "image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

type Download struct {
	Data          []byte
	ContentType   string
	Width, Height int
	Animated      bool
}

// DownloadMedia uses the same DNS-pinned public-only transport and redirect
// validation as page previews. It accepts inert media bytes, never HTML or SVG.
func (s *Service) DownloadMedia(ctx context.Context, raw string, maximum int64) (Download, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for redirects := 0; redirects < 4; redirects++ {
		u, err := s.validate(ctx, raw)
		if err != nil {
			return Download{}, err
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		req.Header.Set("Accept", "image/png,image/jpeg,image/gif,image/webp,video/mp4,video/webm")
		res, err := s.client.Do(req)
		if err != nil {
			return Download{}, errors.New("media host is unavailable")
		}
		if res.StatusCode >= 300 && res.StatusCode < 400 {
			next, err := res.Location()
			res.Body.Close()
			if err != nil {
				return Download{}, errors.New("invalid media redirect")
			}
			raw = next.String()
			continue
		}
		if res.StatusCode != 200 || res.ContentLength > maximum {
			res.Body.Close()
			return Download{}, errors.New("media is unavailable or exceeds the file limit")
		}
		prefix, err := io.ReadAll(io.LimitReader(res.Body, min(int64(512), maximum+1)))
		if err != nil {
			res.Body.Close()
			return Download{}, errors.New("media download failed")
		}
		mime := strings.Split(http.DetectContentType(prefix), ";")[0]
		switch mime {
		case "image/png", "image/jpeg", "image/gif", "image/webp", "video/mp4", "video/webm":
		default:
			res.Body.Close()
			return Download{}, errors.New("only JPEG, PNG, GIF, WebP, MP4 and WebM media are supported")
		}
		data, err := io.ReadAll(io.LimitReader(io.MultiReader(bytes.NewReader(prefix), res.Body), maximum+1))
		res.Body.Close()
		if err != nil || len(data) == 0 || int64(len(data)) > maximum {
			return Download{}, errors.New("media is unavailable or exceeds the file limit")
		}
		out := Download{Data: data, ContentType: mime}
		switch mime {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			cfg, _, err := stdimage.DecodeConfig(bytes.NewReader(data))
			if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 16384 || cfg.Height > 16384 || int64(cfg.Width)*int64(cfg.Height) > 64_000_000 {
				return Download{}, errors.New("invalid media dimensions")
			}
			out.Width, out.Height = cfg.Width, cfg.Height
			out.Animated = mime == "image/gif" || mime == "image/webp" && len(data) > 20 && string(data[12:16]) == "VP8X" && data[20]&2 != 0
			if mime == "image/png" {
				for offset := 8; offset+12 <= len(data); {
					size := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
					if size < 0 || size > len(data)-offset-12 {
						break
					}
					kind := string(data[offset+4 : offset+8])
					if kind == "acTL" {
						out.Animated = true
						break
					}
					if kind == "IDAT" {
						break
					}
					offset += 12 + size
				}
			}
		case "video/mp4", "video/webm":
		default:
			return Download{}, errors.New("only JPEG, PNG, GIF, WebP, MP4 and WebM media are supported")
		}
		return out, nil
	}
	return Download{}, errors.New("too many media redirects")
}
