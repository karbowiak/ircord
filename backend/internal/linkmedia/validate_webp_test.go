package linkmedia

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"testing"
)

func riffChunk(fourCC string, payload []byte) []byte {
	chunk := append([]byte(fourCC), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(chunk[4:], uint32(len(payload)))
	chunk = append(chunk, payload...)
	if len(payload)%2 == 1 {
		chunk = append(chunk, 0)
	}
	return chunk
}

func putUint24(b []byte, v int) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }

type webpFrame struct{ x, y, width, height int }

// animatedWebPFixture builds the container of an animated WebP. Frame bitstreams
// are placeholders: the validator checks the container, never the codec data.
func animatedWebPFixture(width, height int, frames ...webpFrame) []byte {
	vp8x := make([]byte, 10)
	vp8x[0] = 0x02 // animation
	putUint24(vp8x[4:7], width-1)
	putUint24(vp8x[7:10], height-1)
	body := append([]byte("WEBP"), riffChunk("VP8X", vp8x)...)
	body = append(body, riffChunk("ANIM", make([]byte, 6))...)
	for _, f := range frames {
		anmf := make([]byte, 16)
		putUint24(anmf[0:3], f.x/2)
		putUint24(anmf[3:6], f.y/2)
		putUint24(anmf[6:9], f.width-1)
		putUint24(anmf[9:12], f.height-1)
		anmf = append(anmf, riffChunk("VP8L", []byte{0x2f, 0, 0, 0, 0})...)
		body = append(body, riffChunk("ANMF", anmf)...)
	}
	return riffChunk("RIFF", body)
}

func TestAnimatedWebPIsAcceptedAfterContainerChecks(t *testing.T) {
	valid := animatedWebPFixture(40, 30, webpFrame{0, 0, 40, 30}, webpFrame{10, 10, 20, 20})
	config, _, err := image.DecodeConfig(bytes.NewReader(valid))
	if err != nil || config.Width != 40 || config.Height != 30 {
		t.Fatalf("canvas size: %+v %v", config, err)
	}
	many := make([]webpFrame, 101)
	for i := range many {
		many[i] = webpFrame{0, 0, 1, 1}
	}
	for _, tc := range []struct {
		name   string
		data   []byte
		reason string
	}{
		{"valid", valid, ""},
		{"no-frames", animatedWebPFixture(40, 30), "invalid_file"},
		{"frame-outside-canvas", animatedWebPFixture(40, 30, webpFrame{30, 0, 20, 30}), "invalid_file"},
		{"truncated", valid[:len(valid)-4], "invalid_file"},
		{"trailing-data", append(append([]byte(nil), valid...), 0, 0), "invalid_file"},
		{"frame-count", animatedWebPFixture(40, 30, many...), "too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asset := Asset{Kind: "image", ContentType: "image/webp", Size: int64(len(tc.data)), Width: 40, Height: 30}
			err := validateFile(gifValidationFile(t, tc.data), &asset)
			if tc.reason == "" {
				if err != nil {
					t.Fatalf("animated WebP rejected: %v", err)
				}
			} else if failureReason(err) != tc.reason {
				t.Fatalf("got %v, want %s", err, tc.reason)
			}
		})
	}
}

// A camera JPEG can carry more than the sniffed prefix of EXIF and other
// metadata before its frame header.
func TestJPEGWithLargeMetadataIsMeasuredFromTheWholeFile(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 64, 48)), nil); err != nil {
		t.Fatal(err)
	}
	segment := func(size int) []byte {
		s := []byte{0xff, 0xe1, 0, 0}
		binary.BigEndian.PutUint16(s[2:], uint16(size+2))
		return append(s, bytes.Repeat([]byte{'x'}, size)...)
	}
	data := append([]byte{0xff, 0xd8}, segment(60000)...)
	data = append(data, segment(20000)...)
	data = append(data, encoded.Bytes()[2:]...)
	if len(data) <= sniffBytes {
		t.Fatal("fixture metadata does not exceed the sniffed prefix")
	}
	r := NewResolver(ResolverOptions{})
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Request: req}, nil
	})}
	// Without object storage the upload is refused only after validation.
	asset, err := r.Store(context.Background(), Asset{URL: "https://media.example/photo.jpg", Kind: "image"})
	if failureReason(err) != "storage_unavailable" || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("JPEG rejected before storage: %v", err)
	}
	if asset.Width != 64 || asset.Height != 48 || asset.ContentType != "image/jpeg" {
		t.Fatalf("JPEG measured as %+v", asset)
	}
}
