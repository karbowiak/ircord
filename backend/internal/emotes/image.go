package emotes

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"net/http"
	"strings"

	_ "golang.org/x/image/webp"
)

var imageTypes = []string{"image/png", "image/gif", "image/webp", "image/jpeg"}

// maxSide bounds emote dimensions; providers serve 32–128 pixel images.
const maxSide = 1024

type Image struct {
	Data          []byte
	ContentType   string
	Width, Height int
	Animated      bool
}

// validate checks size, sniffed type and dimensions, and detects animation.
func (s *Service) validate(data []byte) (Image, error) {
	if len(data) == 0 {
		return Image{}, fail(ErrInvalid, "Choose an image.")
	}
	if int64(len(data)) > s.options.MaxBytes {
		return Image{}, fail(ErrInvalid, "Emote images are limited to %d KiB.", s.options.MaxBytes>>10)
	}
	contentType := strings.Split(http.DetectContentType(data), ";")[0]
	known := false
	for _, t := range imageTypes {
		known = known || t == contentType
	}
	if !known {
		return Image{}, fail(ErrInvalid, "Emotes must be PNG, GIF, WebP or JPEG images.")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return Image{}, fail(ErrInvalid, "The image could not be read.")
	}
	if cfg.Width > maxSide || cfg.Height > maxSide {
		return Image{}, fail(ErrInvalid, "Emote images are at most %d×%d pixels.", maxSide, maxSide)
	}
	img := Image{Data: data, ContentType: contentType, Width: cfg.Width, Height: cfg.Height}
	switch contentType {
	case "image/gif":
		// Counting frames by walking the block structure never decodes them: GIF frames
		// compress so well that a 1 MiB file can expand to hundreds of MiB.
		frames, ok := gifFrames(data, 2)
		if !ok {
			return Image{}, fail(ErrInvalid, "The image could not be read.")
		}
		img.Animated = frames > 1
	case "image/webp":
		// An extended (VP8X) header flags animation in its first byte.
		img.Animated = len(data) > 20 && string(data[12:16]) == "VP8X" && data[20]&0x02 != 0
	case "image/png":
		img.Animated = apng(data)
	}
	return img, nil
}

// gifFrames counts image descriptors, stopping at limit, without decoding any frame.
// ok is false for a truncated or malformed block structure.
func gifFrames(data []byte, limit int) (frames int, ok bool) {
	if len(data) < 13 {
		return 0, false
	}
	table := func(packed byte) int {
		if packed&0x80 == 0 {
			return 0
		}
		return 3 << ((packed & 0x07) + 1)
	}
	i := 13 + table(data[10])
	subBlocks := func() bool {
		for {
			if i >= len(data) {
				return false
			}
			size := int(data[i])
			i++
			if size == 0 {
				return true
			}
			i += size
		}
	}
	for i < len(data) {
		switch data[i] {
		case 0x21: // extension: introducer, label, sub-blocks
			i += 2
			if !subBlocks() {
				return frames, false
			}
		case 0x2C: // image descriptor (10 bytes), optional local colour table, LZW code size, data
			if i+10 > len(data) {
				return frames, false
			}
			frames++
			if frames >= limit {
				return frames, true
			}
			i += 10 + table(data[i+9]) + 1
			if !subBlocks() {
				return frames, false
			}
		case 0x3B: // trailer
			return frames, frames > 0
		default:
			return frames, false
		}
	}
	return frames, frames > 0
}

// apng reports whether a PNG has an animation control chunk before its image data.
func apng(data []byte) bool {
	for i := 8; i+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[i:]))
		kind := string(data[i+4 : i+8])
		if kind == "acTL" {
			return true
		}
		if kind == "IDAT" || size < 0 || i+12+size > len(data) {
			return false
		}
		i += 12 + size
	}
	return false
}

// gifStill renders a GIF's first frame as a PNG.
func gifStill(img Image) (Image, error) {
	if img.ContentType != "image/gif" {
		return Image{}, fail(ErrInvalid, "Animated images need a still frame.")
	}
	first, err := gif.Decode(bytes.NewReader(img.Data))
	if err != nil {
		return Image{}, fail(ErrInvalid, "The image could not be read.")
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, img.Width, img.Height))
	draw.Draw(canvas, first.Bounds(), first, first.Bounds().Min, draw.Over)
	var out bytes.Buffer
	if err = png.Encode(&out, canvas); err != nil {
		return Image{}, err
	}
	return Image{Data: out.Bytes(), ContentType: "image/png", Width: img.Width, Height: img.Height}, nil
}

func extension(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/jpeg":
		return ".jpg"
	}
	return ""
}
