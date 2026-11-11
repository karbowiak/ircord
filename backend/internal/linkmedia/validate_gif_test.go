package linkmedia

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/storage"
)

func animatedGIFFixture(t *testing.T, width, height, frames int) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	black := image.NewPaletted(image.Rect(0, 0, width, height), palette)
	white := image.NewPaletted(black.Rect, palette)
	for i := range white.Pix {
		white.Pix[i] = 1
	}
	animation := gif.GIF{LoopCount: 0}
	for i := range frames {
		animation.Image = append(animation.Image, []*image.Paletted{black, white}[i%2])
		animation.Delay = append(animation.Delay, 5)
	}
	var data bytes.Buffer
	if err := gif.EncodeAll(&data, &animation); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func gifValidationFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "animation-*.gif")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	return file
}

// These descriptors have empty image data: the preflight can inspect their
// allocation bounds, but DecodeAll must reject them if it is reached.
func gifFrameDescriptors(width, height, frames int) []byte {
	header := []byte("GIF89a\x00\x00\x00\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff")
	binary.LittleEndian.PutUint16(header[6:8], uint16(width))
	binary.LittleEndian.PutUint16(header[8:10], uint16(height))
	descriptor := []byte{0x2c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0}
	binary.LittleEndian.PutUint16(descriptor[5:7], uint16(width))
	binary.LittleEndian.PutUint16(descriptor[7:9], uint16(height))
	return append(append(header, bytes.Repeat(descriptor, frames)...), 0x3b)
}

func TestGIFPreflightAllocationBounds(t *testing.T) {
	// Exactly 64 million pixels can pass the allocation scan without decoding.
	data := gifFrameDescriptors(800, 800, 100)
	if err := validateGIF(gifValidationFile(t, data), int64(len(data))); err != nil {
		t.Fatalf("aggregate boundary rejected: %v", err)
	}
	for _, tc := range []struct {
		name                  string
		width, height, frames int
	}{
		{"aggregate-pixels", 801, 800, 100},
		{"frame-count", 1, 1, 101},
		{"frame-dimension", maxImageDimension + 1, 1, 1},
		{"zero-dimension", 0, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := gifFrameDescriptors(tc.width, tc.height, tc.frames)
			file := gifValidationFile(t, data)
			asset := Asset{Kind: "image", ContentType: "image/gif", Size: int64(len(data))}
			// Empty compressed data would fail DecodeAll as invalid_file. A
			// too_large result proves the guard ran before codec allocation.
			if err := validateFile(file, &asset); !errors.Is(err, ErrLimit) || failureReason(err) != "too_large" {
				t.Fatalf("allocation bound reached decoder: %v", err)
			}
		})
	}
}

func TestGIFWholeFileValidationRejectsMalformedData(t *testing.T) {
	valid := animatedGIFFixture(t, 2, 2, 2)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"valid", valid},
		{"truncated-frame", valid[:len(valid)/2]},
		{"missing-trailer", valid[:len(valid)-1]},
		{"trailing-data", append(append([]byte(nil), valid...), 0)},
		{"invalid-codec-data", gifFrameDescriptors(1, 1, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := gifValidationFile(t, tc.data)
			asset := Asset{Kind: "image", ContentType: "image/gif", Size: int64(len(tc.data))}
			err := validateFile(file, &asset)
			if tc.name == "valid" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrUnavailable) || failureReason(err) != "invalid_file" {
				t.Fatalf("malformed animation accepted: %v", err)
			}
		})
	}
}

func TestLargeGIFPreviewDeliveryAndCacheRetry(t *testing.T) {
	// Match the reported dimensions and frame count, using compact deterministic
	// frames instead of tracking a large third-party binary fixture.
	data := animatedGIFFixture(t, 600, 600, 81)
	verifyGIFPreviewDelivery(t, data, "https://media.example/large.GIF?version=1")
}

func TestReportedGIFPreviewDelivery(t *testing.T) {
	fixture := os.Getenv("IRCORD_TEST_GIF_FILE")
	if fixture == "" {
		t.Skip("set IRCORD_TEST_GIF_FILE to a downloaded GIF for isolated validation")
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	verifyGIFPreviewDelivery(t, data, "https://static.klipy.com/ii/2fd1ee858360694ebc7272832a866c2b/3a/db/Pc5GIHE36gyV3.gif")
}

func verifyGIFPreviewDelivery(t *testing.T, data []byte, raw string) {
	t.Helper()
	config, err := gif.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	asset := Asset{Kind: "image", ContentType: "image/gif", Size: int64(len(data)), Width: config.Width, Height: config.Height}
	if err := validateFile(gifValidationFile(t, data), &asset); err != nil {
		t.Fatalf("whole-file validation: %v", err)
	}
	hash := sha256.Sum256(data)
	wantHash := hex.EncodeToString(hash[:])
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	puts, heads, fetches := 0, 0, 0
	objects := map[string]bool{}
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		res := response(http.StatusOK, "application/xml", "")
		res.Header.Set("ETag", `"fixture"`)
		switch req.Method {
		case http.MethodPut:
			puts++
			gotHash := sha256.New()
			n, err := io.Copy(gotHash, req.Body)
			if err != nil || n != int64(len(data)) || hex.EncodeToString(gotHash.Sum(nil)) != wantHash || req.Header.Get("Content-Type") != "image/gif" || req.Header.Get("Content-Disposition") != "" || !strings.HasPrefix(req.URL.Path, "/fixture/link-media/") {
				t.Errorf("GIF upload lost bytes, MIME type or inline delivery: size=%d error=%v", n, err)
			}
			objects[req.URL.Path] = true
		case http.MethodHead:
			heads++
			if !objects[req.URL.Path] {
				t.Error("cache checked an unknown CDN object")
			}
			res.ContentLength = int64(len(data))
			res.Header.Set("Content-Length", strconv.Itoa(len(data)))
			res.Header.Set("Content-Type", "image/gif")
			res.Header.Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		default:
			t.Errorf("unexpected storage method: %s", req.Method)
		}
		return res, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(ResolverOptions{CDN: client})
	r.persistent = &publicCache{db: store.DB, namespace: client.Namespace()}
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		fetches++
		if req.URL.String() != raw {
			t.Error("unexpected source request")
		}
		// Identification must use the GIF bytes, not the upstream MIME header.
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/octet-stream"}}, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Request: req}, nil
	})}
	stored, err := r.Store(context.Background(), Asset{URL: raw, Kind: "image"})
	if err != nil || stored.ContentType != "image/gif" || stored.Size != int64(len(data)) || stored.SHA256 != wantHash || !r.IsDeliveryURL(stored.URL) {
		t.Fatalf("Store did not publish the validated GIF: %+v error=%v", stored, err)
	}
	// Direct-file rejections used to persist for 24 hours. Seed both caches to
	// prove the GIF-specific key bypasses them without purging existing data.
	until := time.Now().Add(24 * time.Hour)
	legacy := Result{URL: raw, Provider: "file", Preview: &Preview{Kind: "file", Reason: "too_large", ExpiresAt: until, File: &FileInfo{Reason: "too_large"}}}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyHash := sha256.Sum256([]byte("native-v1\x00" + resolutionKey(raw)))
	legacyKey := "native-v1:" + hex.EncodeToString(legacyHash[:])
	if _, err := store.DB.Exec(`INSERT INTO public_media_results(namespace,cache_key,payload,expires_ns) VALUES(?,?,?,?)`, r.persistent.namespace, legacyKey, encoded, until.UnixNano()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	close(done)
	r.cache.entries = map[string]*resolutionEntry{"native-v1:" + resolutionKey(raw): {value: legacy, expires: until, done: done}}
	value, err := r.ResolveSource(context.Background(), raw)
	if err != nil || value.Preview != nil || value.Embed != nil || len(value.Assets) != 1 || puts != 2 || fetches != 2 {
		t.Fatalf("old rejection blocked native GIF preview: assets=%d puts=%d fetches=%d error=%v", len(value.Assets), puts, fetches, err)
	}
	delivered := value.Assets[0]
	if delivered.Kind != "image" || delivered.ContentType != "image/gif" || delivered.Width != config.Width || delivered.Height != config.Height || delivered.Size != int64(len(data)) || delivered.SHA256 != wantHash || !r.IsDeliveryURL(delivered.URL) {
		t.Fatalf("GIF preview metadata or CDN URL lost: %+v", delivered)
	}
	fresh := NewResolver(r.options)
	fresh.persistent = r.persistent
	fresh.http = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		t.Error("durable cache refetched the source")
		return nil, ErrUnavailable
	})}
	reused, err := fresh.ResolveSource(context.Background(), raw)
	if err != nil || len(reused.Assets) != 1 || reused.Assets[0].ObjectKey != delivered.ObjectKey || puts != 2 || heads != 1 {
		t.Fatalf("durable GIF result was not reused: assets=%d puts=%d heads=%d error=%v", len(reused.Assets), puts, heads, err)
	}
	var rows int
	if err := store.DB.QueryRow(`SELECT count(*) FROM public_media_results`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("legacy cache entry was removed: rows=%d error=%v", rows, err)
	}
	if resolutionVersion("https://media.example/photo.png") != "native-v1" || resolutionVersion("https://media.example/movie.mp4") != "native-v1" || resolutionVersion(raw) != "native-gif-v2" {
		t.Fatal("GIF retry changed unrelated cache versions")
	}
	t.Logf("validated and published GIF through isolated CDN: bytes=%d dimensions=%dx%d sha256=%s", len(data), config.Width, config.Height, wantHash)
}
