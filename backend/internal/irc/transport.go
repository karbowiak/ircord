package irc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sync/atomic"
	"time"
)

var errPrivateDestination = errors.New("private IRC destinations are disabled")
var errWebIRCPrivateOnly = errors.New("plaintext WEBIRC requires a private IRC destination")

func HTTPClient(allowPrivate bool) *http.Client {
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (safeDialer{ctx: ctx, allowPrivate: allowPrivate}).Dial(network, address)
	}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxIdleConns: 100, MaxIdleConnsPerHost: 4}}
}

type safeDialer struct {
	connected      func(net.Conn) net.Conn
	ctx            context.Context
	allowPrivate   bool
	requirePrivate bool
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
		if d.requirePrivate && !(ip.IsPrivate() || ip.IsLoopback()) {
			return nil, errWebIRCPrivateOnly
		}
		if !d.allowPrivate && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip)) {
			return nil, errPrivateDestination
		}
	}
	var lastErr error
	for _, ip := range ips {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		lastErr = err
		if err == nil {
			if d.connected != nil {
				conn = d.connected(conn)
			}
			return conn, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("connect to IRC destination: %w", lastErr)
	}
	return nil, fmt.Errorf("IRC destination has no usable addresses")
}

// attemptConn records I/O failure before girc finishes draining its handlers.
// Registration replies already buffered by girc may still be processed.
type attemptConn struct {
	net.Conn
	dead atomic.Bool
}

func (c *attemptConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if err != nil {
		c.dead.Store(true)
	}
	return n, err
}

func (c *attemptConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if err != nil {
		c.dead.Store(true)
	}
	return n, err
}

func (c *attemptConn) Close() error {
	c.dead.Store(true)
	return c.Conn.Close()
}
