package mediafetch

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"testing"
	"time"
)

func TestPublicDestinationsAndExtractorProxy(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "https://user:pass@example.com/a", "http://localhost/a", "https://127.0.0.1/a", "https://[::1]/a", "https://10.0.0.1/a", "https://example.com:9000/a"} {
		if _, err := URL(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.169.254", "192.0.2.1", "fd00::1", "64:ff9b::7f00:1"} {
		if PublicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		if !PublicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("blocked %s", raw)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if conn, err := DialContext(ctx, "tcp", "127.0.0.1:80"); err == nil {
		conn.Close()
		t.Fatal("private dial succeeded")
	}
	proxy, closeProxy, err := Proxy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer closeProxy()
	p, _ := url.Parse(proxy)
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(p)}}
	defer client.CloseIdleConnections()
	res, err := client.Get("http://127.0.0.1/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("private HTTP proxy status: %d", res.StatusCode)
	}
	if res, err := client.Get("https://127.0.0.1/"); err == nil {
		res.Body.Close()
		t.Fatal("private CONNECT succeeded")
	}
	closeProxy()
	if conn, err := net.DialTimeout("tcp", p.Host, 100*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("proxy listener survived close")
	}
}
