package linkmedia

import (
	"encoding/binary"
	"os"
)

// Bound aggregate frame allocations before the standard GIF reader decodes
// them. This only walks block lengths and frame dimensions, not image codecs.
// gif.DecodeAll remains responsible for validating every image frame.
func validateGIF(f *os.File, size int64) error {
	header := make([]byte, 13)
	if _, err := f.ReadAt(header, 0); err != nil {
		return rejectFile("invalid_file", ErrUnavailable)
	}
	at := int64(13)
	if header[10]&0x80 != 0 {
		at += 3 * int64(1<<((header[10]&7)+1))
	}
	frames, pixels := 0, int64(0)
	read := func(n int) ([]byte, error) {
		if at+int64(n) > size {
			return nil, ErrUnavailable
		}
		b := make([]byte, n)
		_, err := f.ReadAt(b, at)
		at += int64(n)
		return b, err
	}
	blocks := func() error {
		for at < size {
			b, err := read(1)
			if err != nil {
				return err
			}
			if b[0] == 0 {
				return nil
			}
			at += int64(b[0])
			if at > size {
				return ErrUnavailable
			}
		}
		return ErrUnavailable
	}
	for at < size {
		b, err := read(1)
		if err != nil {
			break
		}
		switch b[0] {
		case 0x3b:
			if frames > 0 && at == size {
				return nil
			}
			return rejectFile("invalid_file", ErrUnavailable)
		case 0x21:
			if _, err = read(1); err != nil {
				return rejectFile("invalid_file", ErrUnavailable)
			}
			if err = blocks(); err != nil {
				return rejectFile("invalid_file", ErrUnavailable)
			}
		case 0x2c:
			d, err := read(9)
			if err != nil {
				return rejectFile("invalid_file", ErrUnavailable)
			}
			w, h := int64(binary.LittleEndian.Uint16(d[4:6])), int64(binary.LittleEndian.Uint16(d[6:8]))
			frames++
			pixels += w * h
			if w == 0 || h == 0 || w > maxImageDimension || h > maxImageDimension || frames > 100 || pixels > 16_000_000 {
				return rejectFile("too_large", ErrLimit)
			}
			if d[8]&0x80 != 0 {
				at += 3 * int64(1<<((d[8]&7)+1))
			}
			code, err := read(1)
			if err != nil || code[0] < 2 || code[0] > 8 {
				return rejectFile("invalid_file", ErrUnavailable)
			}
			if err = blocks(); err != nil {
				return rejectFile("invalid_file", ErrUnavailable)
			}
		default:
			return rejectFile("invalid_file", ErrUnavailable)
		}
	}
	return rejectFile("invalid_file", ErrUnavailable)
}
