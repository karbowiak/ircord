// Package mediafetch handles public media HTTP requests. Trusted internal
// services such as S3 use their own configured transports.
package mediafetch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"ircord/backend/internal/publicnet"
)

func URL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 8192 || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || (u.Port() != "" && u.Port() != "443" && u.Port() != "80") {
		return nil, errors.New("invalid public media URL")
	}
	host := strings.ToLower(u.Hostname())
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicnet.IsPublic(ip) {
			return nil, errors.New("private media destination blocked")
		}
	} else if host == "localhost" || strings.HasSuffix(host, ".localhost") || !strings.Contains(host, ".") {
		return nil, errors.New("private media destination blocked")
	}
	u.Fragment = ""
	return u, nil
}

// Client has no total timeout: each caller bounds its request with a context
// deadline, so a configured link-media time budget is not cut short. Dialing,
// the TLS handshake and waiting for response headers keep their own limits.
func Client() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy: nil, DialContext: DialContext, MaxIdleConns: 20, MaxIdleConnsPerHost: 2,
		TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxResponseHeaderBytes: 32768,
	}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return errors.New("too many media redirects")
		}
		_, err := URL(req.URL.String())
		return err
	}}
}

func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || (port != "80" && port != "443") {
		return nil, errors.New("invalid public media destination")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("media host has no addresses")
	}
	for _, ip := range ips {
		if !publicnet.IsPublic(ip) {
			return nil, errors.New("private media destination blocked")
		}
	}
	for _, ip := range ips {
		conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, errors.New("media connection failed")
}
