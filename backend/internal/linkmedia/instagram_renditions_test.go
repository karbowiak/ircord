package linkmedia

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"
)

func instagramTestImage(t *testing.T, width, height int) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewGray(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func instagramOversizedHeader(t *testing.T) []byte {
	t.Helper()
	// The prefix advertises the reported original's dimensions. Ingestion must
	// reject it before allocating or decoding any image pixels.
	data := instagramTestImage(t, 2, 3)
	binary.BigEndian.PutUint32(data[16:20], 3273)
	binary.BigEndian.PutUint32(data[20:24], 4096)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func TestInstagramOversizedOriginalUsesCompleteSamePhotoRendition(t *testing.T) {
	puts, deletes := 0, 0
	r, store := newSocialTestResolver(t, &puts, &deletes)
	raw := "https://www.instagram.com/p/DfhCCWHlLFS/?img_index=6&dlrf=MTMwc2w3djJ1MTBxNA=="
	canonical, _, id, _ := socialSource(raw)
	document := `{"id":"DfhCCWHlLFS","extractor_key":"Instagram","entries":[{"extractor_key":"Instagram","formats":[],"thumbnails":[{"url":"https://scontent.cdninstagram.com/rendition.png"},{"url":"https://scontent.cdninstagram.com/crop.png"},{"url":"https://scontent.cdninstagram.com/original.png"}],"thumbnail":"https://scontent.cdninstagram.com/original.png"},{"extractor_key":"Instagram","formats":[],"thumbnails":[{"url":"https://scontent.cdninstagram.com/second.png"}]}]}`
	r.options.Downloader = fakeDownloader(t, document)
	original := instagramOversizedHeader(t)
	crop, rendition := instagramTestImage(t, 80, 80), instagramTestImage(t, 80, 100)
	var fetched []string
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		fetched = append(fetched, req.URL.Path)
		data := rendition
		switch req.URL.Path {
		case "/original.png":
			data = original
		case "/crop.png":
			data = crop
		}
		res := response(200, "image/png", string(data))
		res.Request = req
		return res, nil
	})}
	// A durable failure under the old key must neither satisfy the new request
	// nor require deleting unrelated cache entries.
	legacy := infoCard(raw, "", "", "too_large", time.Minute)
	legacy.Provider, legacy.ID = "instagram", id
	encoded, _ := json.Marshal(legacy)
	legacyHash := sha256.Sum256([]byte("native-social-v1\x00" + canonical))
	legacyKey := "native-social-v1:" + hex.EncodeToString(legacyHash[:])
	if _, err := store.DB.Exec(`INSERT INTO public_media_results(namespace,cache_key,payload,expires_ns) VALUES(?,?,?,?)`, r.persistent.namespace, legacyKey, encoded, time.Now().Add(time.Minute).UnixNano()); err != nil {
		t.Fatal(err)
	}
	value, err := r.ResolveSource(context.Background(), raw)
	if err != nil || len(value.Assets) != 2 || value.URL != raw || puts != 2 || deletes != 0 || strings.Join(fetched, ",") != "/original.png,/crop.png,/rendition.png,/second.png" {
		t.Fatalf("full ordered gallery failed: assets=%d puts=%d deletes=%d requests=%v err=%v", len(value.Assets), puts, deletes, fetched, err)
	}
	if value.Assets[0].Width != 80 || value.Assets[0].Height != 100 || value.Assets[0].downloadBytes != int64(len(original)+len(crop)+len(rendition)) || !r.IsDeliveryURL(value.Assets[0].URL) {
		t.Fatal("cropped rendition, unaccounted downloads, or external delivery")
	}
	var rows int
	if err := store.DB.QueryRow(`SELECT count(*) FROM public_media_results`).Scan(&rows); err != nil || rows != 2 || !strings.HasPrefix(publicResultKey(canonical), "native-instagram-v2:") {
		t.Fatalf("cache migration removed legacy entries or failed to save new result: %d %v", rows, err)
	}
	encoded, _ = json.Marshal(value)
	if strings.Contains(string(encoded), "cdninstagram.com") || value.Assets[0].SourceURL != "" || len(value.Assets[0].imageRenditions) != 0 {
		t.Fatal("transient source data retained")
	}
}

func TestInstagramRenditionsRetainLimitsAndWholeGalleryFailure(t *testing.T) {
	for _, test := range []struct {
		name, reason string
		status       int
		budget       bool
		crop         bool
		tiny         bool
	}{
		{name: "restricted", reason: "restricted", status: 403},
		{name: "wrong-type", reason: "unsupported_type", status: 200},
		{name: "only-crops", reason: "too_large", status: 200, crop: true},
		{name: "tiny-crop", reason: "too_large", status: 200, tiny: true},
		{name: "shared-byte-budget", reason: "too_large", status: 200, budget: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			puts, deletes := 0, 0
			r, _ := newSocialTestResolver(t, &puts, &deletes)
			canonical := "https://www.instagram.com/p/Gallery/"
			asset := instagramPhotoRenditions(Asset{URL: "https://scontent.cdninstagram.com/original.png", Kind: "image"}, []ytImage{{URL: "https://scontent.cdninstagram.com/alternative.png"}})
			r.options.Downloader = connectorFunc(func(context.Context, string) (Result, error) {
				return Result{URL: canonical, Provider: "instagram", ID: "Gallery", Assets: []Asset{{URL: "https://scontent.cdninstagram.com/first.png", Kind: "image"}, asset}}, nil
			})
			original, rendition := instagramOversizedHeader(t), instagramTestImage(t, 80, 100)
			if test.budget {
				r.options.MaxFileBytes = int64(len(original) + len(rendition) - 1)
			}
			if test.crop {
				rendition = instagramTestImage(t, 80, 80)
			}
			if test.tiny {
				rendition = instagramTestImage(t, 1, 1)
			}
			r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				data, status := rendition, http.StatusOK
				switch req.URL.Path {
				case "/first.png":
					data = nativeFixtureBytes(t, "image")
				case "/original.png":
					data = original
				case "/alternative.png":
					status = test.status
					if !test.budget && !test.crop && !test.tiny {
						data = []byte("<html>unavailable</html>")
					}
				}
				res := response(status, "image/png", string(data))
				res.Request = req
				return res, nil
			})}
			value, err := r.ResolveSource(context.Background(), canonical)
			if err != nil || len(value.Assets) != 0 || value.Preview == nil || value.Preview.Reason != test.reason || puts != 1 || deletes != 1 {
				t.Fatalf("failed gallery was published or reason lost: assets=%d preview=%v puts=%d deletes=%d err=%v", len(value.Assets), value.Preview, puts, deletes, err)
			}
		})
	}
}

func TestInstagramPhotoRenditionsStayBoundedAndPublic(t *testing.T) {
	asset := Asset{URL: "https://scontent.cdninstagram.com/original.jpg", Kind: "image"}
	images := make([]ytImage, 40)
	for index := range images {
		images[index].URL = "https://scontent.cdninstagram.com/variant.jpg?index=" + string(rune('A'+index))
	}
	images = append(images, ytImage{URL: asset.URL}, ytImage{URL: "https://evil.example/photo.jpg"}, ytImage{URL: "http://scontent.cdninstagram.com/photo.jpg"})
	asset = instagramPhotoRenditions(asset, images)
	if len(asset.imageRenditions) != 15 {
		t.Fatalf("unbounded or missing candidate list: %d", len(asset.imageRenditions))
	}
	for _, source := range asset.imageRenditions {
		if source == asset.URL || !socialMediaURL(source, "instagram") {
			t.Fatal("duplicate, external or non-HTTPS rendition")
		}
	}
}
