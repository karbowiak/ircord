package preview

import (
	"bytes"
	"context"
	stdimage "image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMediaDownloadValidatesBytesLimitsAndRedirects(t *testing.T) {
	var body bytes.Buffer
	_ = png.Encode(&body, stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 3)))
	calls := 0
	fetch := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credentials forwarded")
		}
		data := body.Bytes()
		status := 200
		headers := http.Header{}
		switch r.URL.Path {
		case "/redirect":
			status = 302
			headers.Set("Location", "https://private.example/file.png")
		case "/spoof.png":
			data = []byte("<html>not an image</html>")
			headers.Set("Content-Type", "image/png")
		case "/doc.png":
			data = []byte("%PDF-1.7 fake")
		case "/movie.mp4":
			data = []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm', 'm', 'p', '4', '2'}
		}
		return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})}})
	got, err := fetch.DownloadMedia(context.Background(), "https://public.example/pixel.png", 1024)
	if err != nil || got.ContentType != "image/png" || got.Width != 2 || got.Height != 3 || !bytes.Equal(got.Data, body.Bytes()) {
		t.Fatal(got, err)
	}
	movie, err := fetch.DownloadMedia(context.Background(), "https://public.example/movie.mp4", 1024)
	if err != nil || movie.ContentType != "video/mp4" {
		t.Fatal(movie.ContentType, err)
	}
	for _, path := range []string{"/spoof.png", "/doc.png", "/redirect"} {
		before := calls
		if _, err = fetch.DownloadMedia(context.Background(), "https://public.example"+path, 1024); err == nil {
			t.Fatal("unsafe bytes/redirect accepted", path)
		}
		if calls != before+1 {
			t.Fatal("private redirect reached transport")
		}
	}
	if _, err = fetch.DownloadMedia(context.Background(), "https://public.example/pixel.png", 8); err == nil {
		t.Fatal("file cap ignored")
	}
	for _, raw := range []string{"http://public.example/pixel.png", "https://private.example/pixel.png", "https://rebind.example/pixel.png", "https://a:b@public.example/pixel.png"} {
		before := calls
		if _, err = fetch.DownloadMedia(context.Background(), raw, 1024); err == nil || calls != before {
			t.Fatalf("unsafe destination reached transport: %s", strings.Split(raw, "@")[len(strings.Split(raw, "@"))-1])
		}
	}
}
