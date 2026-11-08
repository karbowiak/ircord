package push

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"time"
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
				if !publicAddress(ip) {
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

var deniedAddresses = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("2002::/16"),
}

func publicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range deniedAddresses {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
