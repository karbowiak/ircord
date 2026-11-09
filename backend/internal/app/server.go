// Package app owns the long-lived backend and its exclusive data directory.
package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"ircord/backend/internal/activity"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"ircord/backend/internal/admin"
	"ircord/backend/internal/api"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/buildinfo"
	"ircord/backend/internal/emotes"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/model"
	"ircord/backend/internal/push"
	"ircord/backend/internal/storage"
)

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func Run(ctx context.Context) error {
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
	dataDir := env("IRCORD_DATA_DIR", ".data")
	store, err := storage.Open(dataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	ircOptions := irc.Options{Store: store, AllowPrivate: os.Getenv("IRCORD_ALLOW_PRIVATE_IRC") == "true"}
	passwordConfig := []byte(os.Getenv("IRCORD_NETWORK_PASSWORDS"))
	if passwordFile := os.Getenv("IRCORD_NETWORK_PASSWORDS_FILE"); passwordFile != "" {
		if len(passwordConfig) != 0 {
			return fmt.Errorf("configure network passwords using either JSON or a file")
		}
		passwordConfig, err = os.ReadFile(passwordFile)
		if err != nil {
			return fmt.Errorf("cannot read network password configuration")
		}
	}
	if len(passwordConfig) != 0 {
		ircOptions.NetworkPasswords, err = irc.ParseNetworkPasswords(passwordConfig)
		if err != nil {
			return err
		}
	}
	if caFile := os.Getenv("IRCORD_IRC_CA_FILE"); caFile != "" {
		raw, err := os.ReadFile(caFile)
		if err != nil {
			return err
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(raw) {
			return fmt.Errorf("IRC CA file contains no certificates")
		}
		ircOptions.TLSConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	}
	publicURL, err := url.Parse(env("IRCORD_PUBLIC_URL", "http://127.0.0.1:8080"))
	if err != nil || publicURL.Host == "" || (publicURL.Scheme != "http" && publicURL.Scheme != "https") || publicURL.User != nil {
		return fmt.Errorf("IRCORD_PUBLIC_URL must be an HTTP(S) URL without credentials")
	}
	var blob filehost.Blob
	switch env("IRCORD_FILE_STORAGE", "local") {
	case "local":
		blob = filehost.Local{Root: filepath.Join(dataDir, "uploads")}
	case "s3":
		s3, err := filehost.NewS3(os.Getenv("IRCORD_S3_ENDPOINT"), os.Getenv("IRCORD_S3_BUCKET"), env("IRCORD_S3_REGION", "us-east-1"), os.Getenv("IRCORD_S3_ACCESS_KEY"), os.Getenv("IRCORD_S3_SECRET_KEY"), os.Getenv("IRCORD_S3_INSECURE") != "true")
		if err != nil {
			return err
		}
		blob = s3
	case "disabled":
	default:
		return fmt.Errorf("IRCORD_FILE_STORAGE must be local, s3 or disabled")
	}
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
	emoteManagers := []string{}
	for _, name := range strings.Split(os.Getenv("IRCORD_EMOTE_MANAGERS"), ",") {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			emoteManagers = append(emoteManagers, name)
		}
	}
	var pushes *push.Service
	subject := os.Getenv("IRCORD_PUSH_SUBJECT")
	if subject == "" && publicURL.Scheme == "https" {
		subject = publicURL.String()
	}
	if subject != "" && os.Getenv("IRCORD_WEB_PUSH") != "false" {
		pushes, err = push.New(push.Options{Store: store, DataDir: dataDir, Subject: subject})
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
	trustedProxies := []netip.Prefix{}
	for _, raw := range strings.Split(os.Getenv("IRCORD_TRUSTED_PROXIES"), ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("invalid trusted proxy CIDR")
		}
		trustedProxies = append(trustedProxies, prefix)
	}
	registrationKey := os.Getenv("IRCORD_REGISTRATION_KEY")
	if path := os.Getenv("IRCORD_REGISTRATION_KEY_FILE"); registrationKey == "" && path != "" {
		value, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read registration key file: %w", readErr)
		}
		registrationKey = strings.TrimSpace(string(value))
	}
	requireRegistrationKey := os.Getenv("IRCORD_REQUIRE_REGISTRATION_KEY") == "true"
	if requireRegistrationKey && registrationKey == "" {
		return fmt.Errorf("registration key is required but not configured")
	}
	authOptions := auth.Options{TrustedProxies: trustedProxies, SecureCookies: publicURL.Scheme == "https", Registration: os.Getenv("IRCORD_ALLOW_REGISTRATION") != "false", RegistrationKey: registrationKey, RequireRegistrationKey: requireRegistrationKey}
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
	origins := strings.Split(env("IRCORD_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"), ",")
	origins = append(origins, publicURL.Scheme+"://"+publicURL.Host)
	media, err := linkmedia.FromEnv()
	if err != nil {
		return fmt.Errorf("configure link media: %w", err)
	}
	app := api.New(manager, api.Options{Media: media, Store: store, Files: files, Emotes: catalog, EmoteManagers: emoteManagers, Push: pushes, Activity: router, Auth: sessions, Origins: origins, Admin: verifier, Build: buildinfo.Current(), Ergo: associated, PublicURL: publicURL.String()})
	server := &http.Server{Addr: env("IRCORD_LISTEN", "127.0.0.1:8080"), Handler: app.Handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 90 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32768}
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
			slog.Error("Indexing stored history for links failed; retrying", "error", err)
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
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("IRCord API listening", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err = <-serverErrors:
		if err != http.ErrServerClosed {
			stop()
			<-cleanupDone
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = server.Shutdown(shutdown)
	stop()
	<-cleanupDone
	return err
}

// homeNetwork is the connection every new account gets to the associated IRC server: SASL with
// the account's own username and password. IRCORD_ERGO_IRC_TLS=false allows a plaintext local
// server; IRCORD_HOME_NETWORK_CHANNELS lists channels to join.
func homeNetwork(endpoint string) (irc.NetworkConfig, error) {
	host, rawPort, err := net.SplitHostPort(endpoint)
	port, portErr := strconv.Atoi(rawPort)
	if err != nil || portErr != nil {
		return irc.NetworkConfig{}, fmt.Errorf("associated Ergo server must be an IRC host:port")
	}
	cfg := irc.NetworkConfig{ID: "ircord", Name: env("IRCORD_HOME_NETWORK_NAME", "IRCord"), Host: host, Port: port, TLS: os.Getenv("IRCORD_ERGO_IRC_TLS") != "false"}
	for _, channel := range strings.Split(os.Getenv("IRCORD_HOME_NETWORK_CHANNELS"), ",") {
		if channel = strings.TrimSpace(channel); channel != "" {
			cfg.AutoJoin = append(cfg.AutoJoin, channel)
		}
	}
	return cfg, nil
}
