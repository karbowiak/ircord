package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/storage"
)

// contextBlob reads like an S3 object body: every read fails once the context
// passed to Open is done.
type contextBlob struct {
	filehost.Local
	delay time.Duration
}

func (b contextBlob) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	body, err := b.Local.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	return &contextBody{ReadCloser: body, ctx: ctx, delay: b.delay}, nil
}

type contextBody struct {
	io.ReadCloser
	ctx   context.Context
	delay time.Duration
}

func (b *contextBody) Read(p []byte) (int, error) {
	select {
	case <-time.After(b.delay):
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	}
	b.delay = 0
	return b.ReadCloser.Read(p)
}

func TestDownloadStreamsPastTheLookupDeadline(t *testing.T) {
	previous := fileLookupTime
	fileLookupTime = 50 * time.Millisecond
	t.Cleanup(func() { fileLookupTime = previous })

	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err = store.CreateAccount("uploader", "uploader"); err != nil {
		t.Fatal(err)
	}
	// The body arrives well after the lookup deadline, as a slow S3 transfer would.
	blob := contextBlob{Local: filehost.Local{Root: filepath.Join(dir, "files")}, delay: 4 * fileLookupTime}
	files, err := filehost.New(filehost.Options{Store: store, Blob: blob, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	const content = "a document that takes a while to arrive"
	upload, err := files.Upload(context.Background(), "uploader", "network", "notes.txt", []byte(content), files.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	attachment, err := files.Attachment("uploader", "network", upload.ID)
	if err != nil {
		t.Fatal(err)
	}
	link, err := url.Parse(attachment.URL)
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{options: Options{Store: store, Files: files}}
	router := chi.NewRouter()
	router.Get("/files/{id}/{token}/*", s.download)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, link.Path, nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), content) {
		t.Fatalf("download = %d %q, want the whole file", response.Code, response.Body.String())
	}
}
