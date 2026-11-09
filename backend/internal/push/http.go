package push

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"ircord/backend/internal/publicnet"
)

// Push endpoints are untrusted account input. Pin public DNS results when
// dialing so endpoint validation cannot be bypassed with DNS rebinding.
func safeHTTPClient() *http.Client {
	return &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{
		Proxy: nil, MaxIdleConns: 20, MaxIdleConnsPerHost: 2,
		MaxResponseHeaderBytes: 32768, TLSHandshakeTimeout: 5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, errors.New("push host has no addresses")
			}
			for _, ip := range ips {
				if !publicnet.IsPublic(ip) {
					return nil, errors.New("private push destination blocked")
				}
			}
			for _, ip := range ips {
				conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
			}
			return nil, errors.New("push connection failed")
		},
	}}
}
