package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/activity"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

type command struct {
	V     int               `json:"v"`
	Type  string            `json:"type"`
	After uint64            `json:"after"`
	Sync  bool              `json:"sync,omitempty"`
	Tab   string            `json:"tab"`
	State model.TypingState `json:"state,omitempty"`
	irc.SendInput
}

func (s *Server) registerEvents(router chi.Router) {
	router.Get("/api/v1/events", s.events)
	huma.Get(s.API, "/api/v1/events/replay", func(ctx context.Context, in *struct {
		After uint64 `query:"after"`
	}) (*Output[struct {
		Items []irc.Event `json:"items"`
		Valid bool        `json:"valid"`
	}], error) {
		replay, _, valid, stop := s.Manager.Subscribe(owner(ctx), in.After)
		defer stop()
		return result(struct {
			Items []irc.Event `json:"items"`
			Valid bool        `json:"valid"`
		}{replay, valid}, s.Manager.StorageError(owner(ctx)))
	})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	r, unwatch, ok := s.watchSocket(w, r)
	if !ok {
		return
	}
	defer unwatch()
	key := owner(r.Context())
	s.socketsMu.Lock()
	if s.sockets[key] >= 16 {
		s.socketsMu.Unlock()
		writeProblem(w, "too many event connections", http.StatusTooManyRequests)
		return
	}
	s.sockets[key]++
	s.socketsMu.Unlock()
	defer func() {
		s.socketsMu.Lock()
		s.sockets[key]--
		if s.sockets[key] == 0 {
			delete(s.sockets, key)
		}
		s.socketsMu.Unlock()
	}()
	socket, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer socket.CloseNow()
	socket.SetReadLimit(32768)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	write := func(value any) error { return writeSocket(ctx, socket, value) }
	ready := map[string]any{"v": 1, "type": "session.ready", "cursor": s.Manager.Cursor(owner(ctx))}
	if r.URL.Query().Get("sync") == "1" {
		ready["sync"] = true
		ready["capabilities"] = s.capabilities()
	}
	var emoteUpdates <-chan int64
	if s.options.Emotes != nil {
		var stopEmotes func()
		emoteUpdates, stopEmotes = s.options.Emotes.Subscribe()
		defer stopEmotes()
		ready["emoteVersion"] = s.options.Emotes.Version()
	}
	if s.options.Activity != nil {
		ready["routing"] = true
	}
	var deviceUpdates <-chan struct{}
	if s.options.Auth != nil {
		var stopDevices func()
		deviceUpdates, stopDevices = s.options.Auth.SubscribeDevices(owner(ctx))
		defer stopDevices()
	}
	if err = write(ready); err != nil {
		return
	}
	resumeCtx, resumeCancel := context.WithTimeout(ctx, 10*time.Second)
	_, raw, err := socket.Read(resumeCtx)
	resumeCancel()
	if err != nil {
		return
	}
	var resume command
	if json.Unmarshal(raw, &resume) != nil || resume.V != 1 || resume.Type != "session.resume" {
		_ = socket.Close(websocket.StatusPolicyViolation, "resume required")
		return
	}
	var alerts <-chan activity.Alert
	if s.options.Activity != nil {
		if len(resume.Tab) > 64 {
			resume.Tab = resume.Tab[:64]
		}
		sub, stopAlerts := s.options.Activity.Subscribe(owner(ctx), principal(ctx).Session.ID, resume.Tab)
		defer stopAlerts()
		alerts = sub.C
	}
	replay, live, valid, unsubscribe := s.Manager.Subscribe(owner(ctx), resume.After)
	defer unsubscribe()
	ephemeral, stopEphemeral := s.Manager.SubscribeEphemeral(owner(ctx))
	defer stopEphemeral()
	if !valid {
		_ = write(map[string]any{"v": 1, "type": "session.resync", "reason": "cursor is outside retained events"})
		return
	}
	for _, e := range replay {
		if write(e) != nil {
			return
		}
	}
	// Subscribe attaches live delivery under the same lock that captures replay.
	// This barrier precedes every queued live event, including an empty replay.
	if resume.Sync {
		cursor := resume.After
		if len(replay) > 0 {
			cursor = replay[len(replay)-1].Seq
		}
		if write(sessionSynced{V: 1, Type: "session.synced", Cursor: cursor}) != nil {
			return
		}
	}
	commands := make(chan any, 16)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		heartbeat := time.NewTicker(heartbeatInterval)
		defer heartbeat.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-live:
				if !ok {
					_ = write(map[string]any{"v": 1, "type": "session.resync", "reason": "consumer fell behind"})
					return
				}
				if write(e) != nil {
					return
				}
			case <-heartbeat.C:
				if !s.socketHeartbeat(ctx, socket) {
					return
				}
			case e := <-commands:
				if write(e) != nil {
					return
				}
			case alert := <-alerts:
				if write(notificationAlertEvent{V: 1, Type: "notification.alert", Data: alert}) != nil {
					return
				}
			case <-deviceUpdates:
				if write(devicesUpdatedEvent{V: 1, Type: "devices.updated"}) != nil {
					return
				}
			case version := <-emoteUpdates:
				if write(emotesUpdatedEvent{V: 1, Type: "emotes.updated", Data: emoteVersion{version}}) != nil {
					return
				}
			case e, ok := <-ephemeral:
				if !ok || write(e) != nil {
					return
				}
			}
		}
	}()
	defer func() { cancel(); wg.Wait() }()
	invalid := 0
	for {
		_, raw, err = socket.Read(ctx)
		if err != nil {
			return
		}
		var in command
		decodeErr := json.Unmarshal(raw, &in)
		if decodeErr == nil && in.V == 1 && in.Type == "typing.set" && in.State.Valid() && in.ConversationID != "" {
			if err := s.Manager.SetTyping(owner(ctx), in.ConversationID, in.State); err != nil {
				select {
				case commands <- map[string]any{"v": 1, "type": "command.error", "message": err.Error()}:
				case <-ctx.Done():
					return
				}
			}
			continue
		}
		if s.options.Auth != nil || decodeErr != nil || in.V != 1 || in.Type != "message.send" {
			invalid++
			if invalid > 4 {
				_ = socket.Close(websocket.StatusPolicyViolation, "commands use HTTP")
				return
			}
			message := "invalid command"
			if s.options.Auth != nil {
				message = "Use authenticated HTTP endpoints for commands."
			}
			select {
			case commands <- map[string]any{"v": 1, "type": "command.error", "message": message, "clientId": in.ClientID}:
			case <-ctx.Done():
				return
			}
			continue
		}
		if err = s.Manager.Send(owner(ctx), in.SendInput); err != nil {
			select {
			case commands <- map[string]any{"v": 1, "type": "command.error", "clientId": in.ClientID, "message": err.Error()}:
			case <-ctx.Done():
				return
			}
		}
	}
}
