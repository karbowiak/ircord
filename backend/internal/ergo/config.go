package ergo

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Server, URL, Token, TLSServerName string
	Roots                             *x509.CertPool
	Timeout                           time.Duration
}

func FromEnvironment(getenv func(string) string) (*Client, error) {
	o := Options{Server: getenv("IRCORD_ERGO_SERVER"), URL: getenv("IRCORD_ERGO_API_URL"), Token: getenv("IRCORD_ERGO_API_TOKEN"), TLSServerName: getenv("IRCORD_ERGO_API_TLS_SERVER_NAME")}
	file, ca := getenv("IRCORD_ERGO_API_TOKEN_FILE"), getenv("IRCORD_ERGO_API_CA_FILE")
	if o.Server == "" && o.URL == "" && o.Token == "" && file == "" && o.TLSServerName == "" && ca == "" {
		return nil, nil
	}
	if file != "" {
		if o.Token != "" {
			return nil, errors.New("configure the Ergo API token using either a value or a file")
		}
		info, err := os.Stat(file)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("cannot read Ergo API token file")
		}
		f, err := os.Open(file)
		if err != nil {
			return nil, errors.New("cannot read Ergo API token file")
		}
		b, err := io.ReadAll(io.LimitReader(f, 4097))
		f.Close()
		if err != nil || len(b) > 4096 {
			return nil, errors.New("cannot read Ergo API token file (maximum 4096 bytes)")
		}
		o.Token = strings.TrimSpace(string(b))
	}
	if ca != "" {
		b, err := os.ReadFile(ca)
		if err != nil {
			return nil, errors.New("cannot read Ergo API CA file")
		}
		o.Roots, err = x509.SystemCertPool()
		if err != nil {
			o.Roots = x509.NewCertPool()
		}
		if !o.Roots.AppendCertsFromPEM(b) {
			return nil, errors.New("Ergo API CA file contains no certificates")
		}
	}
	return New(o)
}

func New(o Options) (*Client, error) {
	host, port, err := net.SplitHostPort(o.Server)
	n, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || n < 1 || n > 65535 || host == "" || strings.ContainsAny(host, " /\\\r\n\t") {
		return nil, errors.New("associated Ergo server must be an IRC host:port")
	}
	u, err := url.Parse(o.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" {
		return nil, errors.New("Ergo API URL must be an HTTP(S) URL without credentials, query, fragment, or encoded path")
	}
	loopback := strings.EqualFold(u.Hostname(), "localhost")
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil {
		loopback = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return nil, errors.New("Ergo API requires verified HTTPS; plaintext is allowed only on loopback")
	}
	if len(o.Token) < 32 || len(o.Token) > 512 || strings.IndexFunc(o.Token, func(r rune) bool { return r < '!' || r > '~' }) >= 0 {
		return nil, errors.New("Ergo API token must be a high-entropy printable ASCII credential (32 to 512 characters)")
	}
	if strings.ContainsAny(o.TLSServerName, " /\\:\r\n\t") {
		return nil, errors.New("invalid Ergo API TLS server name")
	}
	if o.Timeout <= 0 {
		o.Timeout = 2 * time.Second
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	transport := httpTransport()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: o.Roots, ServerName: o.TLSServerName}
	return &Client{server: o.Server, base: u, token: o.Token, slots: make(chan struct{}, 4), http: newHTTPClient(transport, o.Timeout)}, nil
}
