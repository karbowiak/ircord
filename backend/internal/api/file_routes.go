package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

type uploadSlots struct {
	mu     sync.Mutex
	total  int
	owners map[string]int
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
		if err != nil {
			_, e := result(false, err)
			return nil, e
		}
		return nil, nil
	})
	s.API.OpenAPI().AddOperation(&huma.Operation{OperationID: "upload-file", Method: "POST", Path: "/api/v1/uploads", Summary: "Upload an account-owned file", Description: "Requires the session cookie. Optional networkId selects and scopes the filehost. Files are bounded and MIME-sniffed by the streaming handler.", Parameters: []*huma.Param{{Name: "networkId", In: "query", Schema: &huma.Schema{Type: "string"}}}, RequestBody: &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"multipart/form-data": {Schema: &huma.Schema{Type: "object", Required: []string{"file"}, Properties: map[string]*huma.Schema{"file": {Type: "string", Format: "binary"}}}}}}, Responses: map[string]*huma.Response{"201": {Description: "Uploaded file metadata", Content: map[string]*huma.MediaType{"application/json": {Schema: s.API.OpenAPI().Components.Schemas.Schema(reflect.TypeOf(filehost.Upload{}), true, "Upload")}}}}})
	router.Post("/api/v1/uploads", s.upload)
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
func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	record, body, err := s.options.Files.Download(ctx, chi.URLParam(r, "id"), chi.URLParam(r, "token"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer body.Close()
	disposition := "attachment"
	if strings.HasPrefix(record.ContentType, "image/") || strings.HasPrefix(record.ContentType, "video/") || strings.HasPrefix(record.ContentType, "audio/") {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", record.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": record.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	if seeker, ok := body.(io.ReadSeeker); ok {
		http.ServeContent(w, r, record.Name, time.Time{}, seeker)
		return
	}
	if r.Method != "HEAD" {
		_, _ = io.Copy(w, body)
	}
}
func (s *Server) profile(ctx context.Context, in *ProfileInput) (model.Member, error) {
	p := model.Profile{DisplayName: in.Body.DisplayName, Bio: in.Body.Bio, NameColor: in.Body.NameColor, AvatarURL: in.Body.AvatarURL}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(in.RawBody, &fields)
	if raw, ok := fields["displayName"]; ok && string(raw) == "null" {
		cleared := ""
		p.DisplayName = &cleared
	}
	if _, ok := fields["nameColor"]; !ok {
		saved := s.Manager.Snapshot(owner(ctx)).User.NameColor
		p.NameColor = &saved
	}
	if raw, ok := in.RawAvatar(); ok {
		if string(raw) == "null" {
			empty := ""
			p.AvatarURL = &empty
		} else {
			var value string
			if json.Unmarshal(raw, &value) != nil || len(value) > 180000 {
				return model.Member{}, errors.New("invalid profile image")
			}
			prefix, encoded, ok := strings.Cut(value, ",")
			if !ok || !strings.Contains(prefix, ";base64") || (!strings.HasPrefix(prefix, "data:image/png;") && !strings.HasPrefix(prefix, "data:image/jpeg;") && !strings.HasPrefix(prefix, "data:image/webp;")) {
				return model.Member{}, errors.New("choose a PNG, JPEG or WebP profile image")
			}
			data, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil || len(data) > 128<<10 || len(data) == 0 {
				return model.Member{}, errors.New("profile image exceeds 128 KiB")
			}
			if s.options.Files == nil {
				return model.Member{}, irc.ErrUnsupported
			}
			upload, err := s.options.Files.Upload(ctx, owner(ctx), "", "avatar", data, s.options.Files.AvatarProvider())
			if err != nil {
				return model.Member{}, err
			}
			p.AvatarURL = &upload.URL
		}
	}
	return s.Manager.SetProfile(owner(ctx), p)
}
