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

// A 1×1 lossless (VP8L) bitstream that the WebP decoder accepts.
var vp8lPixel = []byte{0x2f, 0x00, 0x00, 0x00, 0x10, 0x07, 0x10, 0x11, 0x11, 0x88, 0x88, 0xfe, 0x07, 0x00}

type webpFrame struct {
	x, y, width, height int64
	bitstream           []byte
}

// animatedWebPFixture builds an animated WebP whose frames carry the given
// VP8L bitstreams.
func animatedWebPFixture(width, height int64, frames ...webpFrame) []byte {
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
		anmf = append(anmf, riffChunk("VP8L", f.bitstream)...)
		body = append(body, riffChunk("ANMF", anmf)...)
	}
	return riffChunk("RIFF", body)
}

func TestAnimatedWebPIsAcceptedAfterEveryFrameDecodes(t *testing.T) {
	valid := animatedWebPFixture(4, 4, webpFrame{0, 0, 1, 1, vp8lPixel}, webpFrame{2, 2, 1, 1, vp8lPixel})
	config, _, err := image.DecodeConfig(bytes.NewReader(valid))
	if err != nil || config.Width != 4 || config.Height != 4 {
		t.Fatalf("canvas size: %+v %v", config, err)
	}
	many := make([]webpFrame, 101)
	for i := range many {
		many[i] = webpFrame{0, 0, 1, 1, vp8lPixel}
	}
	garbage := bytes.Repeat([]byte{0x2f, 0xff}, 7)
	for _, tc := range []struct {
		name   string
		data   []byte
		reason string
	}{
		{"valid", valid, ""},
		{"no-frames", animatedWebPFixture(4, 4), "invalid_file"},
		{"frame-outside-canvas", animatedWebPFixture(4, 4, webpFrame{4, 0, 1, 1, vp8lPixel}), "invalid_file"},
		{"undecodable-frame", animatedWebPFixture(4, 4, webpFrame{0, 0, 1, 1, vp8lPixel}, webpFrame{2, 2, 1, 1, garbage}), "invalid_file"},
		{"frame-size-mismatch", animatedWebPFixture(4, 4, webpFrame{0, 0, 2, 2, vp8lPixel}), "invalid_file"},
		{"truncated", valid[:len(valid)-4], "invalid_file"},
		{"trailing-data", append(append([]byte(nil), valid...), 0, 0), "invalid_file"},
		{"frame-count", animatedWebPFixture(4, 4, many...), "too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asset := Asset{Kind: "image", ContentType: "image/webp", Size: int64(len(tc.data)), Width: 4, Height: 4}
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
