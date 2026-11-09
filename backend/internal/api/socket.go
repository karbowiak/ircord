package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// heartbeatInterval is how often private sockets are pinged and their sessions rechecked.
var heartbeatInterval = 30 * time.Second

// watchSocket binds both private streams to session revocation before upgrading.
// The common middleware has already authenticated the account and checked its origin.
func (s *Server) watchSocket(w http.ResponseWriter, r *http.Request) (*http.Request, func(), bool) {
	if r.Header.Get("Origin") == "" {
		writeProblem(w, "origin required", http.StatusForbidden)
		return r, nil, false
	}
	if s.options.Auth == nil {
		return r, func() {}, true
	}
	ctx, unwatch, err := s.options.Auth.Watch(r.Context(), principal(r.Context()))
	if err != nil {
		writeProblem(w, "authentication required", http.StatusUnauthorized)
		return r, nil, false
	}
	return r.WithContext(ctx), unwatch, true
}

func writeSocket(ctx context.Context, socket *websocket.Conn, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return socket.Write(deadline, websocket.MessageText, raw)
}

func (s *Server) socketHeartbeat(ctx context.Context, socket *websocket.Conn) bool {
	pingCtx, done := context.WithTimeout(ctx, 5*time.Second)
	err := socket.Ping(pingCtx)
	done()
	if err != nil {
		return false
	}
	// A session revoked without canceling its watchers still loses its socket.
	return s.options.Auth == nil || s.options.Auth.SessionActive(principal(ctx)) && s.options.Auth.TouchSession(principal(ctx)) == nil
}
