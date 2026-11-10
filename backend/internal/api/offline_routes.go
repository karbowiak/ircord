package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/mediafetch"
)

const offlineFileBytes = 32 << 20

func (s *Server) registerOffline(router chi.Router) {
	s.offlineFiles = make(chan struct{}, 4)
	s.offlineHTTP = mediafetch.Client()
	huma.Get(s.API, "/api/v1/offline/snapshot", func(ctx context.Context, in *struct {
		Before string `query:"before" maxLength:"512"`
		Limit  int    `query:"limit" default:"500" minimum:"1" maximum:"500"`
	}) (*Output[irc.OfflineSnapshot], error) {
		page, err := s.Manager.OfflineSnapshot(ctx, owner(ctx), in.Before, in.Limit)
		if err == nil {
			page.Items = s.messagesWithGeometry(ctx, page.Items)
		}
		return result(page, err)
	})
	huma.Post(s.API, "/api/v1/offline/pinned", func(ctx context.Context, in *struct {
		Body struct {
			Items []irc.OfflineMessageRef `json:"items" maxItems:"100"`
		}
	}) (*Output[irc.OfflinePinned], error) {
		page, err := s.Manager.OfflinePinned(ctx, owner(ctx), in.Body.Items)
		if err == nil {
			page.Items = s.messagesWithGeometry(ctx, page.Items)
		}
		return result(page, err)
	})
	router.Get("/api/v1/offline/media", s.offlineMedia)
	// This binary response is described explicitly; the browser stores its bytes.
	s.API.OpenAPI().AddOperation(&huma.Operation{OperationID: "offline-media", Method: "GET", Path: "/api/v1/offline/media", Summary: "Download a visible message file for offline use", Parameters: []*huma.Param{
		{Name: "conversationId", In: "query", Required: true, Schema: &huma.Schema{Type: "string"}},
		{Name: "messageId", In: "query", Required: true, Schema: &huma.Schema{Type: "string"}},
		{Name: "url", In: "query", Required: true, Schema: &huma.Schema{Type: "string"}},
	}, Responses: map[string]*huma.Response{"200": {Description: "Account-authorized file, at most 32 MiB", Content: map[string]*huma.MediaType{"application/octet-stream": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}}}})
}

func (s *Server) offlineMedia(w http.ResponseWriter, r *http.Request) {
	select {
	case s.offlineFiles <- struct{}{}:
		defer func() { <-s.offlineFiles }()
	default:
		writeProblem(w, "offline download capacity reached; try again shortly", 429)
		return
	}
	query := r.URL.Query()
	message, err := s.Manager.Message(owner(r.Context()), query.Get("conversationId"), query.Get("messageId"))
	if err != nil || message.Deleted {
		writeProblem(w, "file not found", 404)
		return
	}
	raw := query.Get("url")
	allowed := message.GIF != nil && (message.GIF.URL == raw || message.GIF.PreviewURL == raw || message.GIF.StillURL == raw) || slices.Contains(linkmedia.URLs(message.Text), raw)
	for _, attachment := range message.AllAttachments() {
		allowed = allowed || attachment.URL == raw
	}
	if !allowed {
		writeProblem(w, "file not found", 404)
		return
	}
	// IRCord-hosted download capabilities can be read directly, without looping
	// through HTTP or broadening the public-media destination policy.
	u, parseErr := url.Parse(raw)
	if parseErr != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		writeProblem(w, "invalid file URL", 400)
		return
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	public, _ := url.Parse(s.options.PublicURL)
	localHost := r.Host
	if public != nil && public.Host != "" {
		localHost = public.Host
	}
	if len(parts) >= 3 && parts[0] == "files" && u.Host == localHost && s.options.Files != nil {
		record, body, downloadErr := s.options.Files.Download(r.Context(), parts[1], parts[2])
		if downloadErr == nil {
			defer body.Close()
			if record.Size > offlineFileBytes {
				writeProblem(w, "file is too large for offline storage", 413)
				return
			}
			w.Header().Set("Content-Type", record.ContentType)
			if s.Manager.AuthorizeConversation(owner(r.Context()), message.ConversationID) != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Content-Disposition", "attachment")
			_, _ = io.Copy(w, io.LimitReader(body, offlineFileBytes))
			return
		}
	}
	if _, err = mediafetch.URL(raw); err != nil {
		writeProblem(w, "file destination unavailable", 400)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), "GET", raw, nil)
	if err != nil {
		writeProblem(w, "invalid file URL", 400)
		return
	}
	response, err := s.offlineHTTP.Do(request)
	if err != nil {
		writeProblem(w, "file download failed", 502)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		writeProblem(w, "file download failed", 502)
		return
	}
	if response.ContentLength > offlineFileBytes {
		writeProblem(w, "file is too large for offline storage", 413)
		return
	}
	// Bound unknown-length responses before sending successful headers.
	data, err := io.ReadAll(io.LimitReader(response.Body, offlineFileBytes+1))
	if err != nil || len(data) > offlineFileBytes {
		writeProblem(w, "file is too large for offline storage", 413)
		return
	}
	if s.Manager.AuthorizeConversation(owner(r.Context()), message.ConversationID) != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "offline-file"))
	_, _ = w.Write(data)
}
