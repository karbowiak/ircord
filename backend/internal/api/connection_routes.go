package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/irc"
)

type LogInput struct {
	ID     string `path:"id"`
	Before string `query:"before"`
	Limit  int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
}

func (s *Server) registerConnectionAPI(router chi.Router) {
	huma.Get(s.API, "/api/v1/networks/{id}/connection", func(ctx context.Context, in *ConversationPath) (*Output[irc.ConnectionInfo], error) {
		return result(s.Manager.ConnectionInfo(owner(ctx), in.ID))
	})
	huma.Get(s.API, "/api/v1/networks/{id}/server-log", func(ctx context.Context, in *LogInput) (*Output[irc.ServerLogPage], error) {
		return result(s.Manager.ServerLog(owner(ctx), in.ID, in.Before, in.Limit, false))
	})
	huma.Get(s.API, "/api/v1/networks/{id}/connection/trace", func(ctx context.Context, in *LogInput) (*Output[irc.ServerLogPage], error) {
		return result(s.Manager.ServerLog(owner(ctx), in.ID, in.Before, in.Limit, true))
	})
	router.Get("/api/v1/networks/{id}/connection/trace/events", s.traceEvents)
}
func (s *Server) traceEvents(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") == "" {
		writeProblem(w, "origin required", 403)
		return
	}
	ctx := r.Context()
	if s.options.Auth != nil {
		watched, unwatch, err := s.options.Auth.Watch(ctx, principal(ctx))
		if err != nil {
			writeProblem(w, "authentication required", 401)
			return
		}
		defer unwatch()
		ctx = watched
	}
	replay, live, valid, stop, err := s.Manager.SubscribeTrace(owner(ctx), chi.URLParam(r, "id"), r.URL.Query().Get("after"))
	if err != nil {
		writeProblem(w, "connection not found", 404)
		return
	}
	defer stop()
	socket, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer socket.CloseNow()
	socket.SetReadLimit(1024)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer cancel()
		if _, _, err := socket.Read(ctx); err == nil {
			_ = socket.Close(websocket.StatusPolicyViolation, "trace is receive-only")
		}
	}()
	write := func(value any) error {
		raw, _ := json.Marshal(value)
		deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return socket.Write(deadline, websocket.MessageText, raw)
	}
	if !valid {
		_ = write(traceResync{V: 1, Type: "connection.resync", Reason: "raw trace cursor is outside the retained window"})
		return
	}
	for _, line := range replay {
		if write(traceLine{V: 1, Type: "connection.line", Data: line}) != nil {
			return
		}
	}
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-live:
			if !ok {
				_ = write(traceResync{V: 1, Type: "connection.resync", Reason: "trace consumer fell behind"})
				return
			}
			if write(traceLine{V: 1, Type: "connection.line", Data: line}) != nil {
				return
			}
		case <-tick.C:
			pingCtx, done := context.WithTimeout(ctx, 5*time.Second)
			err := socket.Ping(pingCtx)
			done()
			if err != nil {
				return
			}
			if s.options.Auth != nil {
				_ = s.options.Auth.Touch(owner(ctx))
			}
		}
	}
}
