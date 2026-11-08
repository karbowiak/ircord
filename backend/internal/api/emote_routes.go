package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/emotes"
)

// emotesUpdatedEvent tells every connected account that the instance catalog changed.
type emotesUpdatedEvent struct {
	V    int          `json:"v" enum:"1"`
	Type string       `json:"type" enum:"emotes.updated"`
	Data emoteVersion `json:"data"`
}

// devicesUpdatedEvent tells an account's connections that its signed-in devices changed;
// clients showing the list read it again. It carries no device data.
type devicesUpdatedEvent struct {
	V    int    `json:"v" enum:"1"`
	Type string `json:"type" enum:"devices.updated"`
}
type emoteVersion struct {
	Version int64 `json:"version"`
}
type EmoteImport struct {
	Provider   string   `json:"provider" enum:"7tv,ffz"`
	ProviderID string   `json:"providerId" minLength:"1" maxLength:"40"`
	Shortcode  string   `json:"shortcode,omitempty" maxLength:"34" doc:"Defaults to the provider's name in lowercase."`
	Aliases    []string `json:"aliases,omitempty" maxItems:"8"`
}

// canManageEmotes applies IRCORD_EMOTE_MANAGERS: account usernames, or * for every account.
func (s *Server) canManageEmotes(ctx context.Context) bool {
	name := strings.ToLower(principal(ctx).Account.Username)
	for _, allowed := range s.options.EmoteManagers {
		if allowed == "*" || name != "" && allowed == name {
			return true
		}
	}
	return false
}
func emoteError(err error) error {
	var detail *emotes.Error
	message := err.Error()
	if errors.As(err, &detail) {
		message = detail.Detail
	}
	switch {
	case errors.Is(err, emotes.ErrNotFound):
		return huma.Error404NotFound(message)
	case errors.Is(err, emotes.ErrConflict):
		return huma.Error409Conflict(message)
	case errors.Is(err, emotes.ErrInvalid):
		return huma.Error422UnprocessableEntity(message)
	case errors.Is(err, emotes.ErrProvider):
		return huma.Error503ServiceUnavailable(message)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return huma.Error503ServiceUnavailable("The emote provider did not answer in time.")
	}
	return huma.Error500InternalServerError("The emote catalog is unavailable.")
}
func (s *Server) emoteManager(ctx context.Context) error {
	if !s.canManageEmotes(ctx) {
		return huma.Error403Forbidden("Only emote managers can change the server's emotes.")
	}
	return nil
}
func (s *Server) emoteSlot() (func(), bool) {
	select {
	case s.emoteSlots <- struct{}{}:
		return func() { <-s.emoteSlots }, true
	default:
		return nil, false
	}
}

func (s *Server) registerEmotes(router chi.Router) {
	service := s.options.Emotes
	if service == nil {
		return
	}
	s.emoteSlots = make(chan struct{}, 2)
	huma.Register(s.API, huma.Operation{OperationID: "get-emotes", Method: "GET", Path: "/api/v1/emotes", Summary: "Read the instance emote catalog", Description: "Every account on this IRCord backend shares the catalog. Images are served by IRCord; rendering never contacts a provider."}, func(ctx context.Context, _ *struct{}) (*Output[emotes.EmoteCatalog], error) {
		return &Output[emotes.EmoteCatalog]{Body: service.Catalog(s.canManageEmotes(ctx))}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "import-emote", Method: "POST", Path: "/api/v1/emotes/imports", DefaultStatus: 201, Summary: "Import a provider emote", Description: "Requires emote management. The provider's image is validated and copied into IRCord's storage; its ID, link and creator are kept for attribution."}, func(ctx context.Context, in *struct{ Body EmoteImport }) (*Output[emotes.Emote], error) {
		if err := s.emoteManager(ctx); err != nil {
			return nil, err
		}
		release, ok := s.emoteSlot()
		if !ok {
			return nil, huma.Error429TooManyRequests("Another emote is being added; try again shortly.")
		}
		defer release()
		e, err := service.Import(ctx, in.Body.Provider, in.Body.ProviderID, emotes.New{Shortcode: in.Body.Shortcode, Aliases: in.Body.Aliases, AddedBy: owner(ctx)})
		if err != nil {
			return nil, emoteError(err)
		}
		return &Output[emotes.Emote]{Body: e}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "update-emote", Method: "PATCH", Path: "/api/v1/emotes/{id}", Summary: "Rename, re-alias, enable or disable an emote", Description: "Requires emote management. Names an emote no longer has are retired: old messages still render it, and another emote may claim them."}, func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Body emotes.EmotePatch
	}) (*Output[emotes.Emote], error) {
		if err := s.emoteManager(ctx); err != nil {
			return nil, err
		}
		e, err := service.Update(in.ID, in.Body)
		if err != nil {
			return nil, emoteError(err)
		}
		return &Output[emotes.Emote]{Body: e}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "delete-emote", Method: "DELETE", Path: "/api/v1/emotes/{id}", DefaultStatus: 204, Summary: "Remove an emote", Description: "Requires emote management. Messages that used it keep their readable :shortcode: text."}, func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*struct{}, error) {
		if err := s.emoteManager(ctx); err != nil {
			return nil, err
		}
		if err := service.Delete(ctx, in.ID); err != nil {
			return nil, emoteError(err)
		}
		return nil, nil
	})
	form := &huma.Schema{Type: "object", Required: []string{"file", "shortcode"}, Properties: map[string]*huma.Schema{
		"file":      {Type: "string", Format: "binary", Description: "PNG, GIF, WebP or JPEG image."},
		"still":     {Type: "string", Format: "binary", Description: "Still frame for an animated WebP or PNG; a GIF's first frame is used otherwise."},
		"shortcode": {Type: "string", MaxLength: ptrInt(34)},
		"name":      {Type: "string", MaxLength: ptrInt(64), Description: "Canonical name; defaults to the shortcode as typed."},
		"aliases":   {Type: "string", MaxLength: ptrInt(300), Description: "Comma-separated aliases."},
		"creator":   {Type: "string", MaxLength: ptrInt(64), Description: "Optional attribution."},
	}}
	s.API.OpenAPI().AddOperation(&huma.Operation{OperationID: "upload-emote", Method: "POST", Path: "/api/v1/emotes", Summary: "Upload a custom emote", Description: "Requires emote management. The image is bounded, MIME-sniffed and stored as an instance asset that outlives the uploader's account.", RequestBody: &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"multipart/form-data": {Schema: form}}}, Responses: map[string]*huma.Response{"201": {Description: "The new emote", Content: map[string]*huma.MediaType{"application/json": {Schema: s.API.OpenAPI().Components.Schemas.Schema(reflect.TypeOf(emotes.Emote{}), true, "Emote")}}}}})
	router.Post("/api/v1/emotes", s.uploadEmote)
	router.Get("/emotes/{file}", s.emoteAsset)
	router.Head("/emotes/{file}", s.emoteAsset)
}
func ptrInt(v int) *int { return &v }

func (s *Server) uploadEmote(w http.ResponseWriter, r *http.Request) {
	if !s.canManageEmotes(r.Context()) {
		writeProblem(w, "Only emote managers can change the server's emotes.", 403)
		return
	}
	release, ok := s.emoteSlot()
	if !ok {
		writeProblem(w, "Another emote is being added; try again shortly.", 429)
		return
	}
	defer release()
	limit := s.options.Emotes.MaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, 2*limit+(64<<10))
	if err := r.ParseMultipartForm(2*limit + (64 << 10)); err != nil {
		writeProblem(w, "invalid multipart upload or image too large", 413)
		return
	}
	defer r.MultipartForm.RemoveAll()
	part := func(name string) ([]byte, error) {
		file, _, err := r.FormFile(name)
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(io.LimitReader(file, limit+1))
	}
	data, err := part("file")
	if err != nil || data == nil {
		writeProblem(w, "Choose an image.", 400)
		return
	}
	still, err := part("still")
	if err != nil {
		writeProblem(w, "The still frame could not be read.", 400)
		return
	}
	aliases := []string{}
	for _, alias := range strings.Split(r.FormValue("aliases"), ",") {
		if alias = strings.TrimSpace(alias); alias != "" {
			aliases = append(aliases, alias)
		}
	}
	e, err := s.options.Emotes.Upload(r.Context(), emotes.New{Name: r.FormValue("name"), Shortcode: r.FormValue("shortcode"), Aliases: aliases, Creator: r.FormValue("creator"), AddedBy: owner(r.Context())}, data, still)
	if err != nil {
		var status huma.StatusError
		if errors.As(emoteError(err), &status) {
			writeProblem(w, status.Error(), status.GetStatus())
			return
		}
		writeProblem(w, "The emote could not be saved.", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_ = json.NewEncoder(w).Encode(e)
}

func (s *Server) emoteAsset(w http.ResponseWriter, r *http.Request) {
	file := chi.URLParam(r, "file")
	key, _, _ := strings.Cut(file, ".")
	if len(key) != 32 || strings.Trim(key, "0123456789abcdef") != "" { // gitleaks:allow -- hexadecimal validation alphabet
		http.NotFound(w, r)
		return
	}
	mime, body, err := s.options.Emotes.Open(r.Context(), key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	// Object keys are random per stored image, so a URL's content never changes.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if r.Method == "HEAD" {
		return
	}
	_, _ = io.Copy(w, body)
}
