package mediacdn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"ircord/backend/internal/filehost"
)

func TestClusterS3RoundTripAndRemoval(t *testing.T) {
	endpoint := os.Getenv("IRCORD_TEST_CDN_ENDPOINT")
	if endpoint == "" {
		t.Skip("set IRCORD_TEST_CDN_ENDPOINT for real S3/CDN validation")
	}
	data := pixel(t, 99)
	s, _, _ := fixture(t, func(r *http.Request) (*http.Response, error) { return nil, nil })
	b, err := filehost.NewS3(endpoint, "ircord-media", "us-east-1", os.Getenv("IRCORD_TEST_CDN_ACCESS_KEY"), os.Getenv("IRCORD_TEST_CDN_SECRET_KEY"), false)
	if err != nil {
		t.Fatal(err)
	}
	b.Prefix = "media/"
	s.options.Blob = b
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	raw := "https://fixture.example/" + hex.EncodeToString(nonce[:]) + ".png"
	// Unique bytes prevent this disposable test from sharing a real user's object.
	data = append(data, nonce[:]...)
	s.options.Fetcher = testFetcher(data)
	record, err := s.Fetch(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Delete(context.Background(), record.Hash)
	reader, _, err := s.Open(context.Background(), record.Hash)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(got) != string(data) {
		t.Fatal("S3 bytes differed", err)
	}
	if public := os.Getenv("IRCORD_TEST_CDN_PUBLIC_URL"); public != "" {
		request, _ := http.NewRequest(http.MethodGet, public+"/"+record.Hash, nil)
		request.Header.Set("User-Agent", "Mozilla/5.0")
		request.Header.Set("Origin", "https://chat.ircord.com")
		request.Header.Set("Range", "bytes=0-7")
		client := &http.Client{Timeout: 15 * time.Second}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 206 || string(body) != string(data[:8]) || response.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("CDN range/CORS failed: status=%d bytes=%d", response.StatusCode, len(body))
		}
	}
	if err = s.Remove(context.Background(), record.Hash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Fetch(context.Background(), raw); err != ErrRemoved {
		t.Fatal("removed S3 content refetched", err)
	}
	if _, _, err = s.Open(context.Background(), record.Hash); err == nil {
		t.Fatal("removed object still served")
	}
}
