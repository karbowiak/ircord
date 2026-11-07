// Package app owns the long-lived backend and its exclusive data directory.
package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"ircord/backend/internal/admin"
	"ircord/backend/internal/api"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/buildinfo"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/preview"
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
	if os.Getenv("IRCORD_LINK_PREVIEWS") == "true" {
		ircOptions.Previews = preview.New(preview.Options{})
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
	ircOptions.Files = files
	manager := irc.New(ircOptions)
	defer manager.Close()
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
	sessions := auth.New(store, auth.Options{TrustedProxies: trustedProxies, SecureCookies: publicURL.Scheme == "https", Registration: os.Getenv("IRCORD_ALLOW_REGISTRATION") != "false", RegistrationKey: registrationKey, RequireRegistrationKey: requireRegistrationKey, BeforeDelete: func(owner string) error {
		if err := manager.DropOwner(owner); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return files.DeleteAccount(ctx, owner)
	}})
	defer sessions.Close()
	if err = sessions.PruneExpired(); err != nil {
		return err
	}
	if err = manager.Restore(); err != nil {
		return err
	}
	origins := strings.Split(env("IRCORD_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"), ",")
	origins = append(origins, publicURL.Scheme+"://"+publicURL.Host)
	app := api.New(manager, api.Options{Store: store, Files: files, Push: pushes, Auth: sessions, Origins: origins, Admin: verifier, Build: buildinfo.Current(), Ergo: associated})
	server := &http.Server{Addr: env("IRCORD_LISTEN", "127.0.0.1:8080"), Handler: app.Handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 90 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32768}
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	cleanupDone := make(chan struct{})
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
