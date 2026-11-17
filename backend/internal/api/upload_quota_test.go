package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"ircord/backend/internal/filehost"
)

func TestUploadPastQuotaIsRefusedWithItsReason(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	files, err := filehost.New(filehost.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(t.TempDir(), "files")}, PublicURL: server.URL, QuotaFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	app.options.Files = files
	client := cookieClient()
	register(t, client, server.URL)
	uploadPNG(t, client, server.URL, "")

	var imageData, body bytes.Buffer
	_ = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 3, 2)))
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "pixel.png")
	part.Write(imageData.Bytes())
	writer.Close()
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var problem struct{ Detail string }
	_ = json.Unmarshal(raw, &problem)
	if res.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(problem.Detail, "limit of 1 uploaded files") {
		t.Fatalf("upload past the quota: %d %s", res.StatusCode, raw)
	}
}

func avatarDataURL(t *testing.T, shade uint8) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: shade, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
}

func TestReplacingProfileImageDeletesTheOldOne(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	avatars := func() []string {
		t.Helper()
		rows, err := store.DB.Query("SELECT id FROM files WHERE owner=? AND backend='avatar'", account.Account.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			ids = append(ids, id)
		}
		return ids
	}
	set := func(value any) string {
		t.Helper()
		var member struct{ AvatarURL string }
		_ = json.Unmarshal(request(t, client, "PATCH", server.URL+"/api/v1/profile", map[string]any{"avatarDataUrl": value}, 200), &member)
		return member.AvatarURL
	}
	first := set(avatarDataURL(t, 1))
	before := avatars()
	if first == "" || len(before) != 1 {
		t.Fatal("first profile image", first, before)
	}
	second := set(avatarDataURL(t, 2))
	after := avatars()
	if second == first || len(after) != 1 || after[0] == before[0] {
		t.Fatal("replaced profile image kept", before, after)
	}
	// A bio edit leaves the image alone.
	request(t, client, "PATCH", server.URL+"/api/v1/profile", map[string]any{"bio": "hello"}, 200)
	if len(avatars()) != 1 {
		t.Fatal("profile edit removed the current image")
	}
	set(nil)
	if left := avatars(); len(left) != 0 {
		t.Fatal("cleared profile image kept", left)
	}
}
