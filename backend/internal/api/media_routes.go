package api

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/admin"
)

func (s *Server) registerMedia(router chi.Router) {
	huma.Register(s.API, huma.Operation{
		OperationID: "admin-remove-media",
		Method:      http.MethodDelete,
		Path:        admin.Prefix + "/media/{hash}",
		Summary:     "Remove a stored media object permanently",
		Description: "Blocks this content hash and its known source URLs from automatic ingestion. Shared uses of this stored copy are also removed. Object-store failures retain the block and are retried by maintenance.",
	}, func(ctx context.Context, in *struct {
		Hash string `path:"hash" pattern:"^[a-f0-9]{64}$"`
	}) (*struct{}, error) {
		if s.options.Media == nil {
			return nil, huma.Error404NotFound("media hosting is disabled")
		}
		ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		err := s.options.Media.Remove(ctx, in.Hash)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("media not found")
		}
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("Media is blocked; object removal will be retried.")
		}
		s.Manager.MediaRemoved(in.Hash)
		return nil, nil
	})
	router.MethodFunc(http.MethodGet, "/media/{hash}", s.downloadMedia)
	router.MethodFunc(http.MethodHead, "/media/{hash}", s.downloadMedia)
}

func (s *Server) downloadMedia(w http.ResponseWriter, r *http.Request) {
	if s.options.Media == nil {
		http.NotFound(w, r)
		return
	}
	reader, record, err := s.options.Media.Open(r.Context(), chi.URLParam(r, "hash"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", record.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
	w.Header().Set("ETag", `"`+record.Hash+`"`)
	if seek, ok := reader.(io.ReadSeeker); ok {
		http.ServeContent(w, r, "media", time.Time{}, seek)
		return
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.CopyN(w, reader, record.Size)
}
