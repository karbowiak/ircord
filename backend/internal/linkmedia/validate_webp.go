package linkmedia

import (
	"encoding/binary"
	"os"
)

// validateAnimatedWebP reports whether a WebP file is animated and, if it is,
// checks its container. Go has no animated WebP decoder, so like GIF this walks
// chunk lengths and frame geometry under the same frame and pixel budgets. The
// browser decodes the frames; still WebP files go through the full decoder.
func validateAnimatedWebP(f *os.File, size int64, canvas Asset) (bool, error) {
	header := make([]byte, 30)
	if _, err := f.ReadAt(header, 0); err != nil {
		return false, rejectFile("invalid_file", ErrUnavailable)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WEBP" {
		return false, rejectFile("invalid_file", ErrUnavailable)
	}
	if string(header[12:16]) != "VP8X" || header[20]&0x02 == 0 {
		return false, nil
	}
	if int64(binary.LittleEndian.Uint32(header[4:8]))+8 != size {
		return true, rejectFile("invalid_file", ErrUnavailable)
	}
	width, height := int64(canvas.Width), int64(canvas.Height)
	at := int64(12)
	seenAnim, frames, pixels := false, 0, int64(0)
	for at < size {
		chunk := make([]byte, 8)
		if at+8 > size {
			return true, rejectFile("invalid_file", ErrUnavailable)
		}
		if _, err := f.ReadAt(chunk, at); err != nil {
			return true, rejectFile("invalid_file", ErrUnavailable)
		}
		length := int64(binary.LittleEndian.Uint32(chunk[4:8]))
		payload := at + 8
		next := payload + length + length&1
		if next > size {
			return true, rejectFile("invalid_file", ErrUnavailable)
		}
		switch string(chunk[0:4]) {
		case "ANIM":
			seenAnim = true
		case "ANMF":
			if !seenAnim || length < 16 {
				return true, rejectFile("invalid_file", ErrUnavailable)
			}
			frame := make([]byte, 12)
			if _, err := f.ReadAt(frame, payload); err != nil {
				return true, rejectFile("invalid_file", ErrUnavailable)
			}
			x, y := 2*uint24(frame[0:3]), 2*uint24(frame[3:6])
			w, h := uint24(frame[6:9])+1, uint24(frame[9:12])+1
			if x+w > width || y+h > height {
				return true, rejectFile("invalid_file", ErrUnavailable)
			}
			frames++
			pixels += w * h
			if frames > 100 || pixels > maxGIFFramePixels {
				return true, rejectFile("too_large", ErrLimit)
			}
		}
		at = next
	}
	if frames == 0 || at != size {
		return true, rejectFile("invalid_file", ErrUnavailable)
	}
	return true, nil
}

func uint24(b []byte) int64 { return int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 }
