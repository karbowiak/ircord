package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/mediafetch"
)

const offlineFileBytes = 32 << 20

// offlineFetchTime bounds one third-party download for offline storage.
const offlineFetchTime = 2 * time.Minute

func (s *Server) registerOffline(router chi.Router) {
	s.offlineFiles = make(chan struct{}, 4)
	s.offlineHTTP = mediafetch.Client()
	s.offlineCache = newOfflineCache(offlineFileBytes, offlineCacheBytes, offlineCacheEntries, offlineCacheTime)
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
	// Viewers of one URL share a single bounded fetch. The copy is saved to disk
	// as it arrives, so a download never holds the whole file in memory.
	original, release, err := s.offlineCache.get(r.Context(), raw, offlineFetchTime, s.fetchOffline(raw))
	if err != nil {
		switch {
		case r.Context().Err() != nil:
		case errors.Is(err, errOfflineBusy):
			writeProblem(w, "offline download capacity reached; try again shortly", 429)
		case errors.Is(err, errOfflineTooLarge):
			writeProblem(w, "file is too large for offline storage", 413)
		default:
			writeProblem(w, "file download failed", 502)
		}
		return
	}
	defer release()
	// Membership can change while the fetch waits, and a shared copy was not
	// fetched for this viewer, so authorize again before sending any bytes.
	if s.Manager.AuthorizeConversation(owner(r.Context()), message.ConversationID) != nil {
		http.NotFound(w, r)
		return
	}
	// The upstream type is not trusted. The saved bytes decide it, and only
	// raster images, video and audio keep their type so the app can show them
	// offline; everything else, including pages and SVG, is opaque.
	w.Header().Set("Content-Type", offlineContentType(original))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(original.Size(), 10))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "offline-file"))
	_, _ = io.Copy(w, original)
}

// fetchOffline returns the download of one third-party original, stopping
// at the size limit.
func (s *Server) fetchOffline(raw string) offlineFetch {
	return func(ctx context.Context, dst io.Writer) error {
		request, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
		if err != nil {
			return err
		}
		response, err := s.offlineHTTP.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return fmt.Errorf("unexpected status %d", response.StatusCode)
		}
		if response.ContentLength > offlineFileBytes {
			return errOfflineTooLarge
		}
		_, err = io.Copy(dst, response.Body)
		return err
	}
}

// offlineContentType names the saved bytes from their signature. Only media
// types a browser plays without running code are sent; the rest stay opaque.
func offlineContentType(file *io.SectionReader) string {
	head := make([]byte, 3072)
	n, _ := file.ReadAt(head, 0)
	kind := mimetype.Detect(head[:n]).String()
	if base, _, err := mime.ParseMediaType(kind); err == nil {
		kind = base
	}
	if kind == "image/avif" || linkmedia.PlayableKind(kind) != "" {
		return kind
	}
	return "application/octet-stream"
}
