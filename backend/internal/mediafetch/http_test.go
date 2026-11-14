package mediafetch

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// A total client timeout would end a large download before the caller's own,
// configurable deadline. Only the connection phases are limited here.
func TestClientLeavesTotalTimeToCaller(t *testing.T) {
	client := Client()
	if client.Timeout != 0 {
		t.Fatalf("client timeout %s overrides the caller's deadline", client.Timeout)
	}
	transport := client.Transport.(*http.Transport)
	if transport.TLSHandshakeTimeout == 0 || transport.ResponseHeaderTimeout == 0 {
		t.Fatal("connection phases are unbounded")
	}
}

func TestPublicDestinationsAndExtractorProxy(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "https://user:pass@example.com/a", "http://localhost/a", "https://127.0.0.1/a", "https://[::1]/a", "https://10.0.0.1/a", "https://example.com:9000/a"} {
		if _, err := URL(raw); err == nil {
			t.Errorf("accepted %s", raw)
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
