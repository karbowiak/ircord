package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/imagevariant"
	"ircord/backend/internal/irc"
)

type uploadSlots struct {
	mu     sync.Mutex
	total  int
	owners map[string]int
}

// ServeContent removes Cache-Control on range and seek errors. Download tokens
// must never become cacheable, including on those error responses.
type fileResponseWriter struct{ http.ResponseWriter }

func (w fileResponseWriter) WriteHeader(status int) {
	w.Header().Set("Cache-Control", "no-store")
	w.ResponseWriter.WriteHeader(status)
}

func (s *Server) provider(ctx context.Context, account, network string) filehost.Provider {
	if s.options.Files == nil {
		return filehost.Provider{Provider: "unavailable", Reason: "File hosting is disabled.", Types: []string{}}
	}
	local := s.options.Files.LocalProvider()
	if local.Available {
		return local
	}
	host, advertised, user, password, err := s.Manager.FilehostConfig(account, network)
	if err != nil {
		return local
	}
	return s.options.Files.Resolve(ctx, host, advertised, user, password)
}
func (s *Server) registerFiles(router chi.Router) {
	if s.options.Files == nil {
		return
	}
	s.uploads.owners = map[string]int{}
	huma.Get(s.API, "/api/v1/networks/{id}/uploads", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*Output[filehost.Provider], error) {
		if _, err := s.Manager.ConnectionInfo(owner(ctx), in.ID); err != nil {
			return result(filehost.Provider{}, err)
		}
		return result(s.provider(ctx, owner(ctx), in.ID), nil)
	})
	huma.Delete(s.API, "/api/v1/uploads/{id}", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*struct{}, error) {
		err := s.options.Files.Delete(ctx, owner(ctx), in.ID)
		if errors.Is(err, filehost.ErrNotFound) {
			err = irc.ErrNotFound
		}
		return empty(err)
	})
	s.API.OpenAPI().AddOperation(&huma.Operation{OperationID: "upload-file", Method: "POST", Path: "/api/v1/uploads", Summary: "Upload an account-owned file", Description: "Requires the session cookie. Optional networkId selects and scopes the filehost. Files are bounded and MIME-sniffed by the streaming handler.", Parameters: []*huma.Param{{Name: "networkId", In: "query", Schema: &huma.Schema{Type: "string"}}}, RequestBody: &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"multipart/form-data": {Schema: &huma.Schema{Type: "object", Required: []string{"file"}, Properties: map[string]*huma.Schema{"file": {Type: "string", Format: "binary"}}}}}}, Responses: map[string]*huma.Response{"201": {Description: "Uploaded file metadata", Content: map[string]*huma.MediaType{"application/json": {Schema: s.API.OpenAPI().Components.Schemas.Schema(reflect.TypeOf(filehost.Upload{}), true, "Upload")}}}}})
	router.Post("/api/v1/uploads", s.upload)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		s.API.OpenAPI().AddOperation(&huma.Operation{
			OperationID: strings.ToLower(method) + "-uploaded-file", Method: method, Path: "/files/{id}/{token}/{filename}",
			Summary:     "Download a file with its download token",
			Description: "The token and original storage are checked on every request, including cached image variants. Anyone with a valid file URL can download it, including party attachments after leaving the party. Optional w and h contain a supported still image inside the requested box without upscaling. Dimensions must be integers from 1 to 2048. Images over 16 MiB or 16 million pixels, animated images, unsupported formats, and images with orientation or color-profile metadata retain their original bytes. Busy image processing also serves the original. Responses are not stored by browsers or shared caches. Byte ranges and HEAD describe the selected representation. Omit w and h to download the original.",
			Parameters: []*huma.Param{
				{Name: "id", In: "path", Required: true, Schema: &huma.Schema{Type: "string"}},
				{Name: "token", In: "path", Required: true, Schema: &huma.Schema{Type: "string"}},
				{Name: "filename", In: "path", Required: true, Schema: &huma.Schema{Type: "string"}},
				{Name: "w", In: "query", Description: "Maximum display width in pixels.", Schema: &huma.Schema{Type: "integer", Minimum: ptrFloat(1), Maximum: ptrFloat(imagevariant.MaxDimension)}},
				{Name: "h", In: "query", Description: "Maximum display height in pixels.", Schema: &huma.Schema{Type: "integer", Minimum: ptrFloat(1), Maximum: ptrFloat(imagevariant.MaxDimension)}},
			},
			Responses: map[string]*huma.Response{
				"200": {Description: "Original file or bounded still-image variant", Content: map[string]*huma.MediaType{"application/octet-stream": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}},
				"206": {Description: "Requested byte range of the selected file representation"},
				"400": {Description: "Invalid or repeated image dimensions"},
				"404": {Description: "File or valid download token not found"},
				"416": {Description: "Byte range outside the selected representation"},
			},
		})
	}
	router.Get("/files/{id}/{token}/*", s.download)
	router.Head("/files/{id}/{token}/*", s.download)
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	account := owner(r.Context())
	s.uploads.mu.Lock()
	if s.uploads.total >= 4 || s.uploads.owners[account] >= 2 {
		s.uploads.mu.Unlock()
		writeProblem(w, "upload capacity reached; try again shortly", 429)
		return
	}
	s.uploads.total++
	s.uploads.owners[account]++
	s.uploads.mu.Unlock()
	defer func() {
		s.uploads.mu.Lock()
		s.uploads.total--
		s.uploads.owners[account]--
		if s.uploads.owners[account] == 0 {
			delete(s.uploads.owners, account)
		}
		s.uploads.mu.Unlock()
	}()
	network := r.URL.Query().Get("networkId")
	if network != "" {
		if _, err := s.Manager.ConnectionInfo(account, network); err != nil {
			writeProblem(w, "network not found", 404)
			return
		}
	}
	provider := s.provider(r.Context(), account, network)
	if !provider.Available {
		writeProblem(w, provider.Reason, 409)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, provider.MaxBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeProblem(w, "invalid multipart upload or file too large", 413)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeProblem(w, "file is required", 400)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, provider.MaxBytes+1))
	if err != nil || int64(len(data)) > provider.MaxBytes {
		writeProblem(w, "file exceeds the upload limit", 413)
		return
	}
	upload, err := s.options.Files.Upload(r.Context(), account, network, header.Filename, data, provider)
	if err != nil {
		writeProblem(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_ = json.NewEncoder(w).Encode(upload)
}

// fileLookupTime bounds opening a stored file and building an image variant.
// Streaming the body afterwards is bounded only by the client connection, so a
// large S3 download is not cut off part way.
var fileLookupTime = 30 * time.Second

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	w = fileResponseWriter{w}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	lookup := time.AfterFunc(fileLookupTime, cancel)
	defer lookup.Stop()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	record, body, err := s.options.Files.Download(ctx, chi.URLParam(r, "id"), chi.URLParam(r, "token"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer body.Close()
	width, height, valid := imageDimensions(r)
	if !valid {
		writeProblem(w, "image dimensions must be integers from 1 to 2048", http.StatusBadRequest)
		return
	}
	var content io.Reader = body
	contentType := record.ContentType
	if (width > 0 || height > 0) && imagevariant.Supported(contentType) &&
		record.Size > 0 && record.Size <= imagevariant.MaxBytes &&
		record.Width > 0 && record.Height > 0 && int64(record.Width)*int64(record.Height) <= imagevariant.MaxPixels {
		var consumed []byte
		tooLarge := errors.New("image exceeds variant input limit")
		variant, err := s.images.Get(ctx, imagevariant.Key{ID: record.ID, Width: width, Height: height}, contentType, func() ([]byte, error) {
			var err error
			consumed, err = io.ReadAll(io.LimitReader(body, imagevariant.MaxBytes+1))
			if err == nil && len(consumed) > imagevariant.MaxBytes {
				err = tooLarge
			}
			return consumed, err
		})
		switch {
		case err == nil:
			content = bytes.NewReader(variant.Data)
			contentType = variant.ContentType
		case errors.Is(err, imagevariant.ErrBusy):
			// Capacity is checked before reading; the original body is untouched.
		case errors.Is(err, tooLarge):
			// Preserve the whole original even if stored size metadata is stale.
			if seeker, ok := body.(io.ReadSeeker); ok {
				if _, err = seeker.Seek(0, io.SeekStart); err != nil {
					writeProblem(w, "file download failed", http.StatusBadGateway)
					return
				}
			} else {
				content = io.MultiReader(bytes.NewReader(consumed), body)
			}
		default:
			writeProblem(w, "file download failed", http.StatusBadGateway)
			return
		}
	}
	if !lookup.Stop() {
		writeProblem(w, "file download failed", http.StatusGatewayTimeout)
		return
	}
	disposition := "attachment"
	if strings.HasPrefix(record.ContentType, "image/") || strings.HasPrefix(record.ContentType, "video/") || strings.HasPrefix(record.ContentType, "audio/") {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": record.Name}))
	if seeker, ok := content.(io.ReadSeeker); ok {
		http.ServeContent(w, r, record.Name, time.Time{}, seeker)
		return
	}
	if r.Method != "HEAD" {
		_, _ = io.Copy(w, content)
	}
}

func imageDimensions(r *http.Request) (width, height int, valid bool) {
	query := r.URL.Query()
	for name, target := range map[string]*int{"w": &width, "h": &height} {
		values, present := query[name]
		if !present {
			continue
		}
		if len(values) != 1 || values[0] == "" || strings.Trim(values[0], "0123456789") != "" {
			return 0, 0, false
		}
		value, err := strconv.Atoi(values[0])
		if err != nil || value < 1 || value > imagevariant.MaxDimension {
			return 0, 0, false
		}
		*target = value
	}
	return width, height, true
}
