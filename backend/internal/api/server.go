// Package api adapts the IRC engine to the browser contract. Identity is injected
// by the host application; an IRC nickname or request body is never an owner ID.
package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/lrstanley/girc"
	"ircord/backend/internal/admin"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/buildinfo"
	"ircord/backend/internal/emotes"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/mediacdn"
	"ircord/backend/internal/model"
	"ircord/backend/internal/push"
	"ircord/backend/internal/storage"
)

type Identity func(http.ResponseWriter, *http.Request) (string, error)
type Options struct {
	Media  *mediacdn.Service
	Ergo   *ergo.Client
	Admin  *admin.Verifier
	Build  buildinfo.Info
	Push   *push.Service
	Files  *filehost.Service
	Emotes *emotes.Service
	// EmoteManagers lists the account usernames that may change the emote catalog; "*" allows every account.
	EmoteManagers []string
	Auth          *auth.Service
	Store         *storage.Store
	Identity      Identity
	Origins       []string
}
type ownerKey struct{}
type Server struct {
	startedAt time.Time
	Manager   *irc.Manager
	Handler   http.Handler
	API       huma.API
	options   Options
	socketsMu sync.Mutex
	sockets   map[string]int
	uploads   uploadSlots
	// emoteSlots bounds concurrent emote uploads and imports.
	emoteSlots chan struct{}
	hashField  hashFieldLimiter
}

func owner(ctx context.Context) string { value, _ := ctx.Value(ownerKey{}).(string); return value }

type Output[T any] struct{ Body T }
type NetworkInput struct{ Body irc.NetworkConfig }
type ConversationInput struct {
	Body struct {
		NetworkID string `json:"networkId"`
		Name      string `json:"name,omitempty"`
		Key       string `json:"key,omitempty"`
		MemberID  string `json:"memberId,omitempty"`
	}
}
type ConversationPath struct {
	ID     string `path:"id"`
	Reason string `query:"reason"`
}
type HistoryInput struct {
	ID     string `path:"id"`
	Before string `query:"before"`
	Around string `query:"around" maxLength:"200" doc:"Read a window centred on this message ID instead of the newest page; cannot be combined with before"`
	Limit  int    `query:"limit" default:"40" minimum:"1" maximum:"100"`
}
type AttractInput struct {
	ID string `path:"id"`
}
type ReactionInput struct {
	ID   string `path:"id"`
	Body struct {
		MessageID string `json:"messageId"`
		Emoji     string `json:"emoji"`
	}
}
type ConversationPatch struct {
	ID   string `path:"id"`
	Body struct {
		Read        *bool   `json:"read,omitempty"`
		UnreadFrom  *string `json:"unreadFrom,omitempty" minLength:"1" maxLength:"200" doc:"Mark the conversation unread from this message on"`
		Favorite    *bool   `json:"favorite,omitempty"`
		PreviewMode *string `json:"previewMode,omitempty" enum:"client,server" doc:"Personal preview mode for a direct message. Channels use channel-settings."`
	}
}
type ChannelSettingsPatch struct {
	ID   string `path:"id"`
	Body struct {
		Notifications *string `json:"notifications,omitempty" enum:"default,all,mentions,none"`
		PreviewMode   *string `json:"previewMode,omitempty" enum:"client,server"`
		// Replaces the whole list; send an empty list to clear it.
		// The 20-host limit applies after de-duplication, so the transport cap is looser.
		TrustedMediaHosts *[]string `json:"trustedMediaHosts,omitempty" nullable:"false" maxItems:"100" doc:"Extra exact host names (such as images.example.com) whose direct media links are embedded in this channel under the trusted preview setting. Replaces the list; at most 20 distinct hosts. Schemes, ports, paths, wildcards, IP addresses and names outside the public DNS are refused."`
	}
}
type NetworkPatch struct {
	ID   string `path:"id"`
	Body NetworkUpdate
}
type NetworkUpdate struct {
	Status   *string          `json:"status,omitempty" enum:"connected,disconnected" nullable:"false"`
	Name     *string          `json:"name,omitempty" nullable:"false" maxLength:"100"`
	Host     *string          `json:"host,omitempty" nullable:"false" maxLength:"253"`
	Port     *int             `json:"port,omitempty" nullable:"false" minimum:"1" maximum:"65535"`
	TLS      *bool            `json:"tls,omitempty" nullable:"false"`
	Nick     *string          `json:"nick,omitempty" nullable:"false" maxLength:"32"`
	AltNick  *string          `json:"altNick,omitempty" nullable:"false" maxLength:"32"`
	Username *string          `json:"username,omitempty" nullable:"false" maxLength:"20"`
	Realname *string          `json:"realname,omitempty" nullable:"false" maxLength:"150"`
	Auth     *model.AuthInput `json:"auth,omitempty"`
	AutoJoin *[]string        `json:"autoJoin,omitempty" nullable:"false"`
	Perform  *[]string        `json:"perform,omitempty" nullable:"false"`
}
type SearchInput struct {
	Q              string `query:"q" minLength:"2" maxLength:"200"`
	ConversationID string `query:"conversationId"`
}
type ProfileInput struct {
	Body    ProfileUpdate
	RawBody []byte
}
type ProfileUpdate struct {
	DisplayName   *string `json:"displayName,omitempty" nullable:"true" maxLength:"100" doc:"Explicit profile name. Omit to keep the current one; send an empty string or null to clear it and fall back to the network real name or nickname."`
	Bio           string  `json:"bio" maxLength:"300"`
	NameColor     *string `json:"nameColor,omitempty" nullable:"true"`
	AvatarDataURL *string `json:"avatarDataUrl,omitempty" nullable:"true" maxLength:"180000"`
	AvatarURL     *string `json:"avatarUrl,omitempty" nullable:"true"`
}

func (in *ProfileInput) RawAvatar() (json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(in.RawBody, &fields)
	v, ok := fields["avatarDataUrl"]
	return v, ok
}

type PreferencesInput struct {
	Body struct {
		Theme       *string `json:"theme,omitempty" enum:"dark,light"`
		Compact     *bool   `json:"compact,omitempty"`
		ShowMembers *bool   `json:"showMembers,omitempty"`
	}
}

func result[T any](value T, err error) (*Output[T], error) {
	if err != nil {
		if errors.Is(err, irc.ErrStorage) {
			return nil, huma.Error503ServiceUnavailable(err.Error())
		}
		if errors.Is(err, irc.ErrUncertain) || errors.Is(err, irc.ErrConflict) || errors.Is(err, irc.ErrSavedLimit) {
			return nil, huma.Error409Conflict(err.Error())
		}
		if errors.Is(err, irc.ErrNotFound) {
			return nil, huma.Error404NotFound(err.Error())
		}
		if errors.Is(err, irc.ErrChannelSetting) {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if errors.Is(err, irc.ErrRateLimited) {
			return nil, huma.Error429TooManyRequests(err.Error())
		}
		if errors.Is(err, irc.ErrForbidden) || errors.Is(err, irc.ErrRedactRefused) || errors.Is(err, irc.ErrNotChannelOperator) || errors.Is(err, irc.ErrAttractNeedsOperator) {
			return nil, huma.Error403Forbidden(err.Error())
		}
		if errors.Is(err, irc.ErrAlreadyDeleted) || errors.Is(err, irc.ErrRedactUnconfirmed) {
			return nil, huma.Error409Conflict(err.Error())
		}
		if errors.Is(err, irc.ErrDisconnected) || errors.Is(err, irc.ErrBusy) {
			return nil, huma.Error409Conflict(err.Error())
		}
		return nil, huma.Error400BadRequest(err.Error())
	}
	return &Output[T]{Body: value}, nil
}
func New(manager *irc.Manager, options Options) *Server {
	if options.Identity == nil && options.Auth == nil {
		panic("API requires an identity provider")
	}
	if options.Build.Version == "" {
		options.Build = buildinfo.Current()
	}
	s := &Server{Manager: manager, options: options, sockets: map[string]int{}, startedAt: time.Now().UTC()}
	router := chi.NewRouter()
	router.Use(s.middleware)
	config := huma.DefaultConfig("IRCord API", "0.1.0")
	config.DocsPath = "/docs"
	config.OpenAPIPath = "/openapi"
	config.DocsRenderer = huma.DocsRendererScalar
	config.DocsRendererConfig = map[string]any{"theme": "purple", "proxyUrl": "", "defaultHttpClient": map[string]string{"targetKey": "js", "clientKey": "fetch"}}
	api := humachi.New(router, config)
	s.API = api
	s.registerAdmin()
	s.registerAuth()
	s.registerCommands()
	s.registerNetworkAPI()
	s.registerServices()
	s.registerAppData()
	s.registerNotifications()
	s.registerPreviews()
	s.registerSaved()
	s.registerLinks()
	s.registerInbox()
	s.registerGallery()
	s.registerHashField()
	s.registerConnectionAPI(router)
	s.registerMedia(router)
	s.registerFiles(router)
	s.registerEmotes(router)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if s.options.Store != nil && s.options.Store.DB.PingContext(r.Context()) != nil {
			writeProblem(w, "storage unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	router.Get("/api/v1/events", s.events)
	huma.Delete(api, "/api/v1/invitations/{id}", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*struct{}, error) {
		if err := manager.RejectInvitation(owner(ctx), in.ID); err != nil {
			_, mapped := result(struct{}{}, err)
			return nil, mapped
		}
		return nil, nil
	})
	huma.Get(api, "/api/v1/bootstrap", func(ctx context.Context, _ *struct{}) (*Output[irc.Snapshot], error) {
		if err := manager.StorageError(owner(ctx)); err != nil {
			return result(irc.Snapshot{}, err)
		}
		snapshot := manager.Snapshot(owner(ctx))
		if s.options.Files != nil {
			p := s.options.Files.LocalProvider()
			if p.Available {
				snapshot.Capabilities.Upload = &model.UploadCapability{MaxBytes: p.MaxBytes, Types: p.Types}
			}
		}
		return result(snapshot, nil)
	})
	huma.Post(api, "/api/v1/networks/check", func(ctx context.Context, in *NetworkCheckInput) (*Output[NetworkCheck], error) {
		err := manager.CheckNetwork(ctx, in.Body.Host, in.Body.Port, in.Body.TLS)
		if err != nil {
			return result(NetworkCheck{Reachable: false, Detail: err.Error()}, nil)
		}
		return result(NetworkCheck{Reachable: true, Detail: "Server address verified. Sign-in is checked when you connect."}, nil)
	})
	huma.Post(api, "/api/v1/networks", func(ctx context.Context, in *NetworkInput) (*Output[irc.Network], error) {
		if in.Body.Nick == "" {
			in.Body.Nick = "irc" + owner(ctx)[:min(8, len(owner(ctx)))]
		}
		return result(manager.Connect(owner(ctx), in.Body))
	})
	huma.Patch(api, "/api/v1/networks/{id}", func(ctx context.Context, in *NetworkPatch) (*Output[irc.Network], error) {
		data, _ := json.Marshal(in.Body)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		if raw, ok := fields["status"]; ok {
			if len(fields) != 1 {
				return result(irc.Network{}, fmt.Errorf("status cannot be combined with configuration"))
			}
			var status string
			if json.Unmarshal(raw, &status) != nil || (status != "connected" && status != "disconnected") {
				return result(irc.Network{}, fmt.Errorf("invalid connection status"))
			}
			var err error
			if status == "connected" {
				err = manager.Reconnect(owner(ctx), in.ID)
			} else {
				err = manager.Disconnect(owner(ctx), in.ID)
			}
			if err != nil {
				return result(irc.Network{}, err)
			}
			for _, n := range manager.Snapshot(owner(ctx)).Networks {
				if n.ID == in.ID {
					return result(n, nil)
				}
			}
			return result(irc.Network{}, irc.ErrNotFound)
		}
		return result(manager.PatchNetwork(owner(ctx), in.ID, fields))
	})
	huma.Post(api, "/api/v1/conversations", func(ctx context.Context, in *ConversationInput) (*Output[irc.Conversation], error) {
		if in.Body.Name != "" {
			return result(manager.JoinKey(owner(ctx), in.Body.NetworkID, in.Body.Name, in.Body.Key))
		}
		for _, member := range manager.Snapshot(owner(ctx)).Members {
			if member.ID == in.Body.MemberID {
				return result(manager.Direct(owner(ctx), in.Body.NetworkID, member.Nick))
			}
		}
		if girc.IsValidNick(in.Body.MemberID) {
			return result(manager.Direct(owner(ctx), in.Body.NetworkID, in.Body.MemberID))
		}
		return result(irc.Conversation{}, irc.ErrNotFound)
	})
	huma.Patch(api, "/api/v1/conversations/{id}", func(ctx context.Context, in *ConversationPatch) (*Output[irc.Conversation], error) {
		// Refuse a channel preview mode before applying read or favorite, so the request is all-or-nothing.
		if in.Body.PreviewMode != nil {
			if err := manager.CheckPreviewMode(owner(ctx), in.ID); err != nil {
				return result(irc.Conversation{}, err)
			}
		}
		if in.Body.UnreadFrom != nil {
			if in.Body.Read != nil && *in.Body.Read {
				return nil, huma.Error422UnprocessableEntity("read and unreadFrom cannot be combined")
			}
			if _, err := manager.MarkUnread(owner(ctx), in.ID, *in.Body.UnreadFrom); err != nil {
				return result(irc.Conversation{}, err)
			}
		}
		conv, err := manager.UpdateConversation(owner(ctx), in.ID, in.Body.Read, in.Body.Favorite)
		if err == nil && in.Body.PreviewMode != nil {
			return result(manager.SetPreviewMode(owner(ctx), in.ID, *in.Body.PreviewMode))
		}
		return result(conv, err)
	})
	huma.Register(api, huma.Operation{OperationID: "update-channel-settings", Method: "PUT", Path: "/api/v1/conversations/{id}/channel-settings", Summary: "Change channel-wide settings", Description: "Changes the notification level, server link previews and extra trusted media hosts for everyone in the channel. Needs channel operator status or higher; every joined IRCord member receives the new values."}, func(ctx context.Context, in *ChannelSettingsPatch) (*Output[irc.Conversation], error) {
		return result(manager.SetChannelSettings(owner(ctx), in.ID, irc.ChannelSettingsInput{Notifications: in.Body.Notifications, PreviewMode: in.Body.PreviewMode, TrustedMediaHosts: in.Body.TrustedMediaHosts}))
	})
	huma.Delete(api, "/api/v1/conversations/{id}", func(ctx context.Context, in *ConversationPath) (*struct{}, error) {
		err := manager.PartReason(owner(ctx), in.ID, in.Reason)
		if err != nil {
			_, e := result(false, err)
			return nil, e
		}
		return nil, nil
	})
	huma.Get(api, "/api/v1/conversations/{id}/messages", func(ctx context.Context, in *HistoryInput) (*Output[irc.MessagePage], error) {
		if in.Around != "" {
			if in.Before != "" {
				return nil, huma.Error422UnprocessableEntity("around and before cannot be combined")
			}
			return result(manager.HistoryAround(owner(ctx), in.ID, in.Around, in.Limit))
		}
		return result(manager.HistoryContext(ctx, owner(ctx), in.ID, in.Before, in.Limit))
	})
	huma.Post(api, "/api/v1/conversations/{id}/reactions", func(ctx context.Context, in *ReactionInput) (*Output[irc.Message], error) {
		return result(manager.ToggleReaction(owner(ctx), in.ID, in.Body.MessageID, in.Body.Emoji))
	})
	huma.Post(api, "/api/v1/conversations/{id}/attract", func(ctx context.Context, in *AttractInput) (*Output[irc.Message], error) {
		return result(manager.Attract(owner(ctx), in.ID))
	})
	huma.Get(api, "/api/v1/search", func(ctx context.Context, in *SearchInput) (*Output[Items[irc.SearchResult]], error) {
		items, err := manager.Search(owner(ctx), in.Q, in.ConversationID)
		return result(Items[irc.SearchResult]{items}, err)
	})
	huma.Patch(api, "/api/v1/profile", func(ctx context.Context, in *ProfileInput) (*Output[irc.Member], error) {
		return result(s.profile(ctx, in))
	})
	if options.Auth == nil {
		huma.Patch(api, "/api/v1/preferences", func(ctx context.Context, in *PreferencesInput) (*Output[irc.Preferences], error) {
			p := manager.Snapshot(owner(ctx)).Preferences
			if in.Body.Theme != nil {
				p.Theme = *in.Body.Theme
			}
			if in.Body.Compact != nil {
				p.Compact = *in.Body.Compact
			}
			if in.Body.ShowMembers != nil {
				p.ShowMembers = *in.Body.ShowMembers
			}
			return result(manager.Preferences(owner(ctx), p), nil)
		})
	}
	s.finalizeSpec()
	s.Handler = router
	return s
}
func (s *Server) originAllowed(r *http.Request) bool {
	raw := r.Header.Get("Origin")
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if raw == scheme+"://"+r.Host {
		return true
	}
	for _, allowed := range s.options.Origins {
		if raw == allowed {
			return true
		}
	}
	return false
}
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		if !s.originAllowed(r) {
			writeProblem(w, "origin rejected", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if admin.IsPath(r.URL.Path) {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
			w.WriteHeader(204)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, httpState{w, r}))
		// Admin credentials never become an account owner or fall back to a cookie.
		// Keep the shared origin check above this boundary for browser callers.
		if admin.IsPath(r.URL.Path) {
			if s.options.Admin == nil {
				writeProblem(w, "administration is not configured", http.StatusServiceUnavailable)
				return
			}
			if !s.options.Admin.Authenticate(r) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="ircord-admin"`)
				writeProblem(w, "admin authentication required", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if s.options.Auth != nil && (r.URL.Path == "/api/v1/auth/signup" || r.URL.Path == "/api/v1/auth/login" || (r.Method == "GET" || r.Method == "HEAD") && (r.URL.Path == "/api/v1/auth/registration" || r.URL.Path == "/api/v1/auth/username")) {
			next.ServeHTTP(w, r)
			return
		}
		var identity string
		var err error
		if s.options.Auth != nil {
			var p auth.Principal
			p, err = s.options.Auth.Authenticate(r)
			if err == nil {
				identity = p.Account.ID
				r = r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
			}
		} else {
			identity, err = s.options.Identity(w, r)
		}
		if err != nil && !errors.Is(err, auth.ErrSession) {
			writeProblem(w, "authentication storage unavailable", 503)
			return
		}
		if r.URL.Path == "/api/v1/auth/session" && s.options.Auth != nil && identity == "" {
			next.ServeHTTP(w, r)
			return
		}
		if err != nil || identity == "" {
			writeProblem(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			if err := s.Manager.StorageError(identity); err != nil {
				writeProblem(w, "workspace storage unavailable", 503)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ownerKey{}, identity)))
	})
}

type command struct {
	V     int               `json:"v"`
	Type  string            `json:"type"`
	After uint64            `json:"after"`
	State model.TypingState `json:"state,omitempty"`
	irc.SendInput
}

// heartbeatInterval is how often an event socket is pinged and its session rechecked.
var heartbeatInterval = 30 * time.Second

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") == "" {
		writeProblem(w, "origin required", 403)
		return
	}
	if s.options.Auth != nil {
		ctx, unwatch, err := s.options.Auth.Watch(r.Context(), principal(r.Context()))
		if err != nil {
			writeProblem(w, "authentication required", 401)
			return
		}
		defer unwatch()
		r = r.WithContext(ctx)
	}
	key := owner(r.Context())
	s.socketsMu.Lock()
	if s.sockets[key] >= 16 {
		s.socketsMu.Unlock()
		writeProblem(w, "too many event connections", 429)
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
	write := func(v any) error {
		body, _ := json.Marshal(v)
		writeCtx, done := context.WithTimeout(ctx, 5*time.Second)
		defer done()
		return socket.Write(writeCtx, websocket.MessageText, body)
	}
	ready := map[string]any{"v": 1, "type": "session.ready", "cursor": s.Manager.Snapshot(owner(ctx)).Cursor}
	var emoteUpdates <-chan int64
	if s.options.Emotes != nil {
		var stopEmotes func()
		emoteUpdates, stopEmotes = s.options.Emotes.Subscribe()
		defer stopEmotes()
		ready["emoteVersion"] = s.options.Emotes.Version()
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
				pingCtx, done := context.WithTimeout(ctx, 5*time.Second)
				err := socket.Ping(pingCtx)
				done()
				if err != nil {
					return
				}
				// Defence in depth: a session revoked without its watchers being cancelled
				// still loses its socket within one heartbeat.
				if s.options.Auth != nil && (!s.options.Auth.SessionActive(principal(ctx)) || s.options.Auth.TouchSession(principal(ctx)) != nil) {
					return
				}
			case e := <-commands:
				if write(e) != nil {
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
				if !ok {
					return
				}
				if write(e) != nil {
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

// DevelopmentSessions preserves the frontend demo's opaque-cookie boundary.
// It is deliberately replaceable and provides no login or durable accounts.
func DevelopmentSessions() Identity {
	var mu sync.Mutex
	sessions := map[string]time.Time{}
	return func(w http.ResponseWriter, r *http.Request) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		if cookie, err := r.Cookie("ircord_dev"); err == nil {
			if expires, ok := sessions[cookie.Value]; ok && now.Before(expires) {
				sessions[cookie.Value] = now.Add(24 * time.Hour)
				digest := sha256.Sum256([]byte(cookie.Value))
				return hex.EncodeToString(digest[:]), nil
			}
		}
		for token, expires := range sessions {
			if now.After(expires) {
				delete(sessions, token)
			}
		}
		if len(sessions) >= 500 {
			return "", fmt.Errorf("development session capacity reached")
		}
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return "", err
		}
		token := hex.EncodeToString(raw[:])
		sessions[token] = now.Add(24 * time.Hour)
		http.SetCookie(w, &http.Cookie{Name: "ircord_dev", Value: token, Path: "/api", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: 86400})
		digest := sha256.Sum256([]byte(token))
		return hex.EncodeToString(digest[:]), nil
	}
}
