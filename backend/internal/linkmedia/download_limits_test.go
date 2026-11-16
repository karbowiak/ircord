package linkmedia

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type countingReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

func TestNoStorageSkipsMediaDownloads(t *testing.T) {
	r := NewResolver(ResolverOptions{})
	r.http = noSourceNetwork(t)
	// A known image asset, such as a thumbnail, is not requested at all.
	_, err := r.Store(context.Background(), Asset{URL: "https://media.example/thumb.png", Kind: "image"})
	if failureReason(err) != "storage_unavailable" || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unstored thumbnail: %v", err)
	}

	// A file link reads only the prefix it needs to tell a page from a file.
	body := append([]byte(decodablePNG(t)), bytes.Repeat([]byte{0}, 4<<20)...)
	var read atomic.Int64
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(countingReader{bytes.NewReader(body), &read}), ContentLength: int64(len(body)), Request: req}, nil
	})}
	out, err := r.ResolveSource(context.Background(), "https://media.example/large.png")
	if err != nil || out.Preview == nil || out.Preview.Reason != "storage_unavailable" || len(out.Assets) != 0 {
		t.Fatalf("unstored file link: %+v %v", out, err)
	}
	if got := read.Load(); got > sniffBytes {
		t.Fatalf("read %d bytes without storage, want at most %d", got, sniffBytes)
	}
}

func TestImageDecodesWaitForASlot(t *testing.T) {
	png := decodablePNG(t)
	r, counts := newStoredTestResolver(t, 1024)
	r.http = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, "image/png", png), nil
	})}
	for range cap(imageDecodes) {
		imageDecodes <- struct{}{}
	}
	release := func() {
		for len(imageDecodes) > 0 {
			<-imageDecodes
		}
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.Store(ctx, Asset{URL: "https://source.example/busy.png", Kind: "image"}); !errors.Is(err, context.DeadlineExceeded) || counts.puts != 0 {
		t.Fatalf("decode ran without a slot: puts=%d %v", counts.puts, err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := r.Store(context.Background(), Asset{URL: "https://source.example/queued.png", Kind: "image"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("decode finished while every slot was taken: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	<-imageDecodes
	if err := <-done; err != nil || counts.puts != 1 {
		t.Fatalf("queued decode: puts=%d %v", counts.puts, err)
	}
}
