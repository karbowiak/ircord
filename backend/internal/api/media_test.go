package api

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ircord/backend/internal/admin"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/mediacdn"
	"ircord/backend/internal/preview"
)

func TestStoredMediaPublicRangesAndOperatorRemoval(t *testing.T) {
	base, _, store, sessions := authenticatedApp(t)
	var imageBytes bytes.Buffer
	_ = png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 3)))
	fetcher := preview.New(preview.Options{Resolver: publicResolver{}, Client: &http.Client{Transport: previewTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(imageBytes.Bytes())), Request: r}, nil
	})}})
	media, err := mediacdn.New(mediacdn.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(t.TempDir(), "media")}, Fetcher: fetcher, PublicURL: "https://cdn.example/media"})
	if err != nil {
		t.Fatal(err)
	}
	record, err := media.Fetch(context.Background(), "https://image.example/pixel.png")
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("ab", 32)
	verifier, _ := admin.NewVerifier(token)
	app := New(base.Manager, Options{Media: media, Store: store, Auth: sessions, Admin: verifier})
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	user := cookieClient()
	register(t, user, server.URL)
	url := server.URL + "/media/" + record.Hash
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Range", "bytes=0-7")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 206 || !bytes.Equal(body, imageBytes.Bytes()[:8]) || response.Header.Get("Access-Control-Allow-Origin") != "*" || response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("public range/CORS mismatch", response.StatusCode, response.Header)
	}
	for _, client := range []*http.Client{http.DefaultClient, user} {
		request(t, client, "DELETE", server.URL+admin.Prefix+"/media/"+record.Hash, nil, 401)
	}
	req, _ = http.NewRequest(http.MethodDelete, server.URL+admin.Prefix+"/media/"+record.Hash, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	response, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatal("operator removal failed", response.StatusCode)
	}
	request(t, http.DefaultClient, "GET", url, nil, 404)
	if _, err = media.Fetch(context.Background(), "https://image.example/pixel.png"); err != mediacdn.ErrRemoved {
		t.Fatal("operator removal allowed refetch", err)
	}
	security := app.API.OpenAPI().Paths[admin.Prefix+"/media/{hash}"].Delete.Security
	if len(security) != 1 || security[0]["adminBearer"] == nil {
		t.Fatal("operator API declared wrong authentication")
	}
}
