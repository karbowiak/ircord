// Package imagevariant produces bounded display copies of already-authorized
// files. It never fetches URLs or persists derivatives.
package imagevariant

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"math"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	MaxDimension = 2048
	MaxBytes     = 16 << 20
	MaxPixels    = 16_000_000
)

// Supported excludes formats whose animation or rendering rules we cannot
// preserve. Metadata within these formats is checked before decoding pixels.
func Supported(contentType string) bool {
	return contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/webp"
}

type Image struct {
	Data        []byte
	ContentType string
}

// resize contains the image within the requested box without upscaling. Formats
// with animation, orientation or color-profile metadata retain their original
// bytes: the Go decoders do not preserve all of those browser rendering rules.
func resize(data []byte, contentType string, width, height int) (Image, bool) {
	original := Image{Data: data, ContentType: contentType}
	if width < 0 || height < 0 || width > MaxDimension || height > MaxDimension ||
		width == 0 && height == 0 || len(data) > MaxBytes || !safeMetadata(data, contentType) {
		return original, false
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || "image/"+format != contentType || config.Width < 1 || config.Height < 1 ||
		config.Width > 16384 || config.Height > 16384 ||
		int64(config.Width)*int64(config.Height) > MaxPixels {
		return original, false
	}
	scale := 1.0
	if width > 0 {
		scale = min(scale, float64(width)/float64(config.Width))
	}
	if height > 0 {
		scale = min(scale, float64(height)/float64(config.Height))
	}
	// Bound both output dimensions even for an extremely tall/wide image.
	scale = min(scale, float64(MaxDimension)/float64(max(config.Width, config.Height)))
	if scale >= 1 {
		return original, false
	}
	w := max(1, int(math.Floor(float64(config.Width)*scale)))
	h := max(1, int(math.Floor(float64(config.Height)*scale)))
	source, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format || source.Bounds().Dx() != config.Width || source.Bounds().Dy() != config.Height {
		return original, false
	}
	output := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.BiLinear.Scale(output, output.Bounds(), source, source.Bounds(), draw.Src, nil)
	var encoded bytes.Buffer
	typeName := "image/png"
	if format == "jpeg" {
		typeName = "image/jpeg"
		err = jpeg.Encode(&encoded, output, &jpeg.Options{Quality: 82})
	} else {
		err = png.Encode(&encoded, output)
	}
	if err != nil || encoded.Len() >= len(data) {
		return original, false
	}
	return Image{Data: encoded.Bytes(), ContentType: typeName}, true
}

func safeMetadata(data []byte, contentType string) bool {
	switch contentType {
	case "image/jpeg":
		if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
			return false
		}
		inScan, sawScan := false, false
		for pos := 2; pos < len(data); {
			if inScan {
				// Progressive images can carry metadata between scans. Do not stop
				// checking at the first SOS marker or interpret stuffed bytes as tags.
				next := bytes.IndexByte(data[pos:], 0xff)
				if next < 0 {
					return false
				}
				pos += next
			}
			if data[pos] != 0xff {
				return false
			}
			for pos < len(data) && data[pos] == 0xff {
				pos++
			}
			if pos >= len(data) {
				return false
			}
			marker := data[pos]
			pos++
			if marker == 0x00 || marker >= 0xd0 && marker <= 0xd7 {
				if !inScan {
					return false
				}
				continue
			}
			if marker == 0xd9 {
				return sawScan && pos == len(data)
			}
			if marker == 0xd8 || marker == 0x01 {
				return false
			}
			if pos+2 > len(data) {
				return false
			}
			size := int(binary.BigEndian.Uint16(data[pos:]))
			if size < 2 || size > len(data)-pos {
				return false
			}
			segment := data[pos+2 : pos+size]
			if marker >= 0xe0 && marker <= 0xef {
				// Only the decoder's JFIF/Adobe color conventions are supported.
				// Preserve EXIF, XMP, ICC, multi-picture and unknown app metadata.
				if !(marker == 0xe0 && (bytes.HasPrefix(segment, []byte("JFIF\x00")) || bytes.HasPrefix(segment, []byte("JFXX\x00"))) ||
					marker == 0xee && bytes.HasPrefix(segment, []byte("Adobe"))) {
					return false
				}
			}
			inScan = marker == 0xda
			sawScan = sawScan || inScan
			pos += size
		}
	case "image/png":
		if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			return false
		}
		for pos := 8; pos+12 <= len(data); {
			size := uint64(binary.BigEndian.Uint32(data[pos:]))
			if size+12 > uint64(len(data)-pos) {
				return false
			}
			switch string(data[pos+4 : pos+8]) {
			case "acTL", "fcTL", "fdAT", "eXIf", "iCCP", "gAMA", "cHRM", "sRGB", "cICP", "mDCV", "cLLI":
				return false
			case "IEND":
				return size == 0 && pos+12 == len(data)
			}
			pos += int(size) + 12
		}
	case "image/webp":
		if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" ||
			uint64(binary.LittleEndian.Uint32(data[4:]))+8 != uint64(len(data)) {
			return false
		}
		for pos := 12; pos+8 <= len(data); {
			size := uint64(binary.LittleEndian.Uint32(data[pos+4:]))
			padded := size + size%2
			if padded+8 > uint64(len(data)-pos) {
				return false
			}
			switch string(data[pos : pos+4]) {
			case "ANIM", "ANMF", "EXIF", "ICCP", "XMP ":
				return false
			case "VP8X":
				if size != 10 || data[pos+8]&0x2e != 0 { // ICC, EXIF, XMP or animation flags.
					return false
				}
			}
			pos += int(padded) + 8
			if pos == len(data) {
				return true
			}
		}
	}
	return false
}
