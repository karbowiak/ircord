package app

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ircord/backend/internal/auth"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
)

type config struct {
	dataDir, listen string
	publicURL       *url.URL
	irc             irc.Options
	auth            auth.Options
	blob            filehost.Blob
	media           *linkmedia.Resolver
	emoteManagers   []string
	origins         []string
	pushSubject     string
	webPush         bool
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// loadConfig validates configuration before storage and long-lived workers start.
// Client constructors here perform no network requests or filesystem mutations.
func loadConfig() (config, error) {
	c := config{dataDir: env("IRCORD_DATA_DIR", ".data"), listen: env("IRCORD_LISTEN", "127.0.0.1:8080")}
	c.irc.AllowPrivate = os.Getenv("IRCORD_ALLOW_PRIVATE_IRC") == "true"
	passwordConfig := []byte(os.Getenv("IRCORD_NETWORK_PASSWORDS"))
	if passwordFile := os.Getenv("IRCORD_NETWORK_PASSWORDS_FILE"); passwordFile != "" {
		if len(passwordConfig) != 0 {
			return config{}, fmt.Errorf("configure network passwords using either JSON or a file")
		}
		var err error
		passwordConfig, err = os.ReadFile(passwordFile)
		if err != nil {
			return config{}, fmt.Errorf("cannot read network password configuration")
		}
	}
	if len(passwordConfig) != 0 {
		var err error
		c.irc.NetworkPasswords, err = irc.ParseNetworkPasswords(passwordConfig)
		if err != nil {
			return config{}, err
		}
	}
	var err error
	c.irc.ServerList, err = irc.LoadServerList(os.Getenv("IRCORD_SERVER_LIST_FILE"), c.irc.NetworkPasswords, c.irc.AllowPrivate)
	if err != nil {
		return config{}, err
	}
	if debug, _ := strconv.ParseBool(os.Getenv("GIRC_DEBUG")); debug && c.irc.ServerList.HasWebIRC() {
		return config{}, fmt.Errorf("GIRC_DEBUG cannot be enabled with WEBIRC")
	}
	if caFile := os.Getenv("IRCORD_IRC_CA_FILE"); caFile != "" {
		raw, err := os.ReadFile(caFile)
		if err != nil {
			return config{}, err
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(raw) {
			return config{}, fmt.Errorf("IRC CA file contains no certificates")
		}
		c.irc.TLSConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	}
	publicURL, err := url.Parse(env("IRCORD_PUBLIC_URL", "http://127.0.0.1:8080"))
	if err != nil || publicURL.Host == "" || (publicURL.Scheme != "http" && publicURL.Scheme != "https") || publicURL.User != nil {
		return config{}, fmt.Errorf("IRCORD_PUBLIC_URL must be an HTTP(S) URL without credentials")
	}
	c.publicURL = publicURL
	switch env("IRCORD_FILE_STORAGE", "local") {
	case "local":
		c.blob = filehost.Local{Root: filepath.Join(c.dataDir, "uploads")}
	case "s3":
		c.blob, err = filehost.NewS3(os.Getenv("IRCORD_S3_ENDPOINT"), os.Getenv("IRCORD_S3_BUCKET"), env("IRCORD_S3_REGION", "us-east-1"), os.Getenv("IRCORD_S3_ACCESS_KEY"), os.Getenv("IRCORD_S3_SECRET_KEY"), os.Getenv("IRCORD_S3_INSECURE") != "true")
		if err != nil {
			return config{}, err
		}
	case "disabled":
	default:
		return config{}, fmt.Errorf("IRCORD_FILE_STORAGE must be local, s3 or disabled")
	}
	for _, name := range strings.Split(os.Getenv("IRCORD_EMOTE_MANAGERS"), ",") {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			c.emoteManagers = append(c.emoteManagers, name)
		}
	}
	c.pushSubject = os.Getenv("IRCORD_PUSH_SUBJECT")
	if c.pushSubject == "" && publicURL.Scheme == "https" {
		c.pushSubject = publicURL.String()
	}
	c.webPush = os.Getenv("IRCORD_WEB_PUSH") != "false"
	trustedProxies := []netip.Prefix{}
	for _, raw := range strings.Split(os.Getenv("IRCORD_TRUSTED_PROXIES"), ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return config{}, fmt.Errorf("invalid trusted proxy CIDR")
		}
		trustedProxies = append(trustedProxies, prefix)
	}
	registrationKey := os.Getenv("IRCORD_REGISTRATION_KEY")
	if path := os.Getenv("IRCORD_REGISTRATION_KEY_FILE"); registrationKey == "" && path != "" {
		value, readErr := os.ReadFile(path)
		if readErr != nil {
			return config{}, fmt.Errorf("read registration key file: %w", readErr)
		}
		registrationKey = strings.TrimSpace(string(value))
	}
	requireRegistrationKey := os.Getenv("IRCORD_REQUIRE_REGISTRATION_KEY") == "true"
	if requireRegistrationKey && registrationKey == "" {
		return config{}, fmt.Errorf("registration key is required but not configured")
	}
	c.auth = auth.Options{TrustedProxies: trustedProxies, SecureCookies: publicURL.Scheme == "https", Registration: os.Getenv("IRCORD_ALLOW_REGISTRATION") != "false", RegistrationKey: registrationKey, RequireRegistrationKey: requireRegistrationKey}
	c.origins = strings.Split(env("IRCORD_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"), ",")
	c.origins = append(c.origins, publicURL.Scheme+"://"+publicURL.Host)
	c.media, err = linkmedia.FromEnv()
	if err != nil {
		return config{}, fmt.Errorf("configure link media: %w", err)
	}
	return c, nil
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

// moderationLogin reads the Ergo operator entry that carries out server owners'
// deletions: IRCORD_ERGO_MODERATION_OPERATOR names it and
// IRCORD_ERGO_MODERATION_PASSWORD_FILE holds its password. Both unset turns
// server-owner deletion off; setting only one is a configuration error.
func moderationLogin(getenv func(string) string) (*irc.ModerationLogin, error) {
	name, path := strings.TrimSpace(getenv("IRCORD_ERGO_MODERATION_OPERATOR")), getenv("IRCORD_ERGO_MODERATION_PASSWORD_FILE")
	if name == "" && path == "" {
		return nil, nil
	}
	if name == "" || path == "" {
		return nil, fmt.Errorf("IRCORD_ERGO_MODERATION_OPERATOR and IRCORD_ERGO_MODERATION_PASSWORD_FILE must be set together")
	}
	if strings.ContainsAny(name, " \r\n") {
		return nil, fmt.Errorf("IRCORD_ERGO_MODERATION_OPERATOR must be one word")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read moderation operator password file: %w", err)
	}
	password := strings.TrimSpace(string(raw))
	if password == "" || strings.ContainsAny(password, " \r\n") {
		return nil, fmt.Errorf("the moderation operator password file must hold one password without spaces")
	}
	return &irc.ModerationLogin{Operator: name, Password: password}, nil
}
