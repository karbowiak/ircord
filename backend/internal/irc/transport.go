package irc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

func HTTPClient(allowPrivate bool) *http.Client {
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (safeDialer{ctx: ctx, allowPrivate: allowPrivate}).Dial(network, address)
	}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxIdleConns: 100, MaxIdleConnsPerHost: 4}}
}

type safeDialer struct {
	connected    func(net.Conn)
	ctx          context.Context
	allowPrivate bool
}

func (d safeDialer) Dial(network, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(d.ctx, 8*time.Second)
	defer cancel()
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		ip = ip.Unmap()
		if !d.allowPrivate && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip)) {
			return nil, fmt.Errorf("private IRC destinations are disabled")
		}
	}
	var lastErr error
	for _, ip := range ips {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		lastErr = err
		if err == nil {
			if d.connected != nil {
				d.connected(conn)
			}
			return conn, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("connect to IRC destination: %w", lastErr)
	}
	return nil, fmt.Errorf("IRC destination has no usable addresses")
}
