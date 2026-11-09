// Package app owns the long-lived backend and its exclusive data directory.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"ircord/backend/internal/activity"
	"ircord/backend/internal/admin"
	"ircord/backend/internal/api"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/buildinfo"
	"ircord/backend/internal/emotes"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/push"
	"ircord/backend/internal/storage"
)

func Run(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	// Validate credentials before acquiring storage or starting IRC.
	token, err := admin.LoadToken(os.Getenv("IRCORD_ADMIN_TOKEN"), os.Getenv("IRCORD_ADMIN_TOKEN_FILE"))
	if err != nil {
		return err
	}
	verifier, err := admin.NewVerifier(token)
	if err != nil {
		return err
	}
	associated, err := ergo.FromEnvironment(os.Getenv)
	if err != nil {
		return err
	}
	if associated != nil {
		defer associated.Close()
	}
	dataDir := cfg.dataDir
	store, err := storage.Open(dataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	ircOptions := cfg.irc
	ircOptions.Store = store
	publicURL, blob := cfg.publicURL, cfg.blob
	files, err := filehost.New(filehost.Options{Store: store, Blob: blob, AvatarBlob: filehost.Local{Root: filepath.Join(dataDir, "avatars")}, PublicURL: strings.TrimRight(publicURL.String(), "/"), Client: irc.HTTPClient(ircOptions.AllowPrivate), TrustedHosts: strings.Split(os.Getenv("IRCORD_FILEHOST_TRUSTED_HOSTS"), ",")})
	if err != nil {
		return err
	}
	// Emote images share the configured file storage, under their own keys and lifecycle.
	// With file hosting disabled they use the same local directory as local storage, so
	// switching between local and disabled keeps them.
	emoteBlob := blob
	if emoteBlob == nil {
		emoteBlob = filehost.Local{Root: filepath.Join(dataDir, "uploads")}
	}
	catalog, err := emotes.Open(emotes.Options{Store: store, Blob: emoteBlob, PublicURL: strings.TrimRight(publicURL.String(), "/"), Client: irc.HTTPClient(ircOptions.AllowPrivate)})
	if err != nil {
		return err
	}
	var pushes *push.Service
	if cfg.pushSubject != "" && cfg.webPush {
		pushes, err = push.New(push.Options{Store: store, DataDir: dataDir, Subject: cfg.pushSubject})
		if err != nil {
			return err
		}
		defer pushes.Close()
	}
	ircOptions.Push = pushes
	intervals, err := activity.NewSQLStore(store.DB)
	if err != nil {
		return err
	}
	routerOptions := activity.Options{Store: intervals}
	if pushes != nil {
		routerOptions.Push = pushes
	}
	router := activity.New(routerOptions)
	ircOptions.Router = router
	ircOptions.Files = files
	if associated != nil && os.Getenv("IRCORD_ERGO_ACCOUNTS") != "false" && os.Getenv("IRCORD_REGISTER_CREATED_CHANNELS") != "false" {
		if home, err := homeNetwork(associated.Server()); err == nil {
			ircOptions.Home = home
		}
	}
	manager := irc.New(ircOptions)
	defer manager.Close()
	router.SetSource(manager)
	authOptions := cfg.auth
	if associated != nil && os.Getenv("IRCORD_ERGO_ACCOUNTS") != "false" {
		home, err := homeNetwork(associated.Server())
		if err != nil {
			return err
		}
		authOptions.Accounts = associated
		authOptions.Provision = func(owner, username, password string) error {
			cfg := home
			cfg.Nick = username
			cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: username, Password: &password}
			return manager.ProvisionHome(owner, cfg)
		}
		authOptions.PasswordChanged = func(owner, password string) error {
			var username string
			if err := store.DB.QueryRow("SELECT username FROM accounts WHERE id=? AND deleted=0", owner).Scan(&username); err != nil {
				return err
			}
			return manager.UpdateHomePassword(owner, home, username, password)
		}
	}
	authOptions.SetDisplayName = func(owner, name string) error {
		_, err := manager.SetProfile(owner, model.Profile{DisplayName: &name})
		return err
	}
	authOptions.BeforeDelete = func(owner string) error {
		if err := manager.DropOwner(owner); err != nil {
			return err
		}
		router.Forget(owner)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return files.DeleteAccount(ctx, owner)
	}
	authOptions.SessionEnded = router.EndSession
	authOptions.SignedOut = manager.SignOut
	authOptions.SignedIn = manager.SignIn
	sessions := auth.New(store, authOptions)
	defer sessions.Close()
	if err = sessions.PruneExpired(); err != nil {
		return err
	}
	// Server owners named in IRCORD_OWNERS (#165). A name only ever resolves to an account that
	// already existed when it was first configured; other owners are granted in Administration.
	unresolved, err := store.ApplyConfiguredOwners(strings.Split(os.Getenv("IRCORD_OWNERS"), ","))
	if err != nil {
		return err
	}
	for _, name := range unresolved {
		slog.Warn("IRCORD_OWNERS names an account that didn't exist when it was configured; grant it with the admin API instead", "username", name)
	}
	if err = manager.Restore(); err != nil {
		return err
	}
	for _, owner := range manager.Owners() {
		router.Track(owner)
	}
	router.Start()
	defer router.Close()
	if err := sessions.RecoverCredentials(ctx); err != nil {
		slog.Error("Credential recovery will retry", "error", err)
	}
	recoveryCtx, stopRecovery := context.WithCancel(ctx)
	defer stopRecovery()
	recoveryDone := make(chan struct{})
	defer func() { stopRecovery(); <-recoveryDone }()
	go func() {
		defer close(recoveryDone)
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-recoveryCtx.Done():
				return
			case <-tick.C:
				if err := sessions.RecoverCredentials(recoveryCtx); err != nil {
					slog.Error("Credential recovery will retry", "error", err)
				}
			}
		}
	}()
	app := api.New(manager, api.Options{Media: cfg.media, Store: store, Files: files, Emotes: catalog, EmoteManagers: cfg.emoteManagers, Push: pushes, Activity: router, Auth: sessions, Origins: cfg.origins, Admin: verifier, Build: buildinfo.Current(), Ergo: associated, PublicURL: publicURL.String()})
	server := &http.Server{Addr: cfg.listen, Handler: app.Handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 90 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32768}
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	if os.Getenv("IRCORD_EMOTE_SEED") != "false" {
		seeded := catalog.SeedInBackground(ctx, emotes.DefaultSeeds)
		defer func() { <-seeded }()
	}
	cleanupDone := make(chan struct{})
	backfillDone := make(chan struct{})
	defer func() { stop(); <-backfillDone }()
	go func() {
		defer close(backfillDone)
		store.BackfillLinks(ctx, 200*time.Millisecond, func(err error) {
			slog.Error("Indexing stored history for search and links failed; retrying", "error", err)
		})
	}()
	go func() {
		defer close(cleanupDone)
		tick := time.NewTicker(time.Hour)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := sessions.PruneExpired(); err != nil {
					slog.Error("Account cleanup failed", "error", err)
				}
				if err := store.PruneChannelCache(time.Now().Add(-24 * time.Hour)); err != nil {
					slog.Error("Channel cache cleanup failed", "error", err)
				}
				if err := store.PruneEvents(time.Now().Add(-30 * 24 * time.Hour)); err != nil {
					slog.Error("Event retention cleanup failed", "error", err)
				}
				if err := store.PruneInbox(time.Now()); err != nil {
					slog.Error("Inbox retention cleanup failed", "error", err)
				}
			}
		}
	}()
	err = serve(ctx, server)
	stop()
	<-cleanupDone
	return err
}
