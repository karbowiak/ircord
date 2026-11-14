// Package api adapts the IRC engine to the browser contract. Identity is injected
// by the host application; an IRC nickname or request body is never an owner ID.
package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"ircord/backend/internal/activity"
	"ircord/backend/internal/admin"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/buildinfo"
	"ircord/backend/internal/emotes"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/imagevariant"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/model"
	"ircord/backend/internal/push"
	"ircord/backend/internal/storage"
)

type Identity func(http.ResponseWriter, *http.Request) (string, error)
type Options struct {
	Ergo      *ergo.Client
	Admin     *admin.Verifier
	Build     buildinfo.Info
	Push      *push.Service
	Files     *filehost.Service
	Emotes    *emotes.Service
	Media     *linkmedia.Resolver
	PublicURL string
	// EmoteManagers lists the account usernames that may change the emote catalog; "*" allows every account.
	EmoteManagers []string
	Auth          *auth.Service
	// Activity routes alerts across the account's devices; nil leaves alerts to each client.
	Activity *activity.Router
	Store    *storage.Store
	Identity Identity
	Origins  []string
}

type Server struct {
	startedAt time.Time
	Manager   *irc.Manager
	Handler   http.Handler
	API       huma.API
	options   Options

	socketsMu    sync.Mutex
	sockets      map[string]int
	uploads      uploadSlots
	offlineFiles chan struct{}
	offlineHTTP  *http.Client
	offlineCache *offlineCache
	// emoteSlots bounds concurrent emote uploads and imports.
	emoteSlots chan struct{}
	hashField  hashFieldLimiter
	linkMedia  linkMediaCache
	images     imagevariant.Cache
}

func New(manager *irc.Manager, options Options) *Server {
	if options.Identity == nil && options.Auth == nil {
		panic("API requires an identity provider")
	}
	if options.Build.Version == "" {
		options.Build = buildinfo.Current()
	}
	s := &Server{Manager: manager, options: options, sockets: map[string]int{}, startedAt: time.Now().UTC()}
	if options.Activity != nil && options.Store != nil {
		options.Activity.SetDevices(options.Store)
	}
	router := chi.NewRouter()
	router.Use(s.middleware)
	config := huma.DefaultConfig("IRCord API", "0.1.0")
	config.DocsPath = "/docs"
	config.OpenAPIPath = "/openapi"
	config.DocsRenderer = huma.DocsRendererScalar
	config.DocsRendererConfig = map[string]any{"theme": "purple", "proxyUrl": "", "defaultHttpClient": map[string]string{"targetKey": "js", "clientKey": "fetch"}}
	s.API = humachi.New(router, config)
	s.registerAdmin()
	s.registerAdministration()
	s.registerAuth()
	s.registerCommands()
	s.registerModeration()
	s.registerNetworkAPI()
	s.registerServices()
	s.registerAppData()
	s.registerNotifications()
	s.registerActivity()
	s.registerSaved()
	s.registerLinks()
	s.registerInbox()
	s.registerHashField()
	s.registerConnectionAPI(router)
	s.registerFiles(router)
	s.registerEmotes(router)
	s.registerLinkMedia(router)
	s.registerConversations()
	s.registerParties()
	s.registerPeople()
	s.registerSearch()
	s.registerProfile()
	s.registerEvents(router)
	s.registerOffline(router)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if s.options.Store != nil && s.options.Store.DB.PingContext(r.Context()) != nil {
			writeProblem(w, "storage unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	huma.Get(s.API, "/api/v1/bootstrap", func(ctx context.Context, _ *struct{}) (*Output[irc.Snapshot], error) {
		if err := manager.StorageError(owner(ctx)); err != nil {
			return result(irc.Snapshot{}, err)
		}
		snapshot := manager.Snapshot(owner(ctx))
		snapshot.Capabilities = s.capabilities()
		snapshot.Capabilities.Parties = manager.PartyCapability(owner(ctx))
		snapshot.Capabilities.Moderation = manager.ModerationCapability(owner(ctx))
		return result(snapshot, nil)
	})
	s.finalizeSpec()
	s.Handler = router
	return s
}

func (s *Server) capabilities() model.Capabilities {
	capabilities := model.Capabilities{}
	if s.options.Files != nil {
		p := s.options.Files.LocalProvider()
		if p.Available {
			capabilities.Upload = &model.UploadCapability{MaxBytes: p.MaxBytes, Types: p.Types}
		}
	}
	return capabilities
}
