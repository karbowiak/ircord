package mediafetch

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// Proxy gives subprocess extractors the same public-DNS-only egress as Go
// downloads, including HTTPS CONNECT requests and redirected media sources.
// Its lifetime is bound to ctx; it never listens beyond loopback.
func Proxy(ctx context.Context) (string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	client := Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 32768}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			upstream, err := DialContext(ctx, "tcp", r.Host)
			if err != nil {
				http.Error(w, "destination blocked or unavailable", http.StatusBadGateway)
				return
			}
			h, ok := w.(http.Hijacker)
			if !ok {
				upstream.Close()
				http.Error(w, "tunnel unavailable", 500)
				return
			}
			downstream, buffered, err := h.Hijack()
			if err != nil {
				upstream.Close()
				return
			}
			defer upstream.Close()
			defer downstream.Close()
			stop := context.AfterFunc(ctx, func() { upstream.Close(); downstream.Close() })
			defer stop()
			if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
				return
			}
			if buffered.Flush() != nil {
				return
			}
			done := make(chan struct{})
			go func() { _, _ = io.Copy(upstream, buffered); upstream.Close(); close(done) }()
			_, _ = io.Copy(downstream, upstream)
			downstream.Close()
			<-done
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost {
			http.Error(w, "method blocked", http.StatusMethodNotAllowed)
			return
		}
		if _, err := URL(r.URL.String()); err != nil {
			http.Error(w, "destination blocked", http.StatusForbidden)
			return
		}
		req := r.Clone(ctx)
		req.RequestURI = ""
		req.Header.Del("Proxy-Authorization")
		req.Header.Del("Proxy-Connection")
		res, err := client.Do(req)
		if err != nil {
			http.Error(w, "destination blocked or unavailable", http.StatusBadGateway)
			return
		}
		defer res.Body.Close()
		for key, values := range res.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	})
	go func() { _ = server.Serve(listener) }()
	stop := context.AfterFunc(ctx, func() { _ = server.Close(); client.CloseIdleConnections() })
	closeProxy := func() { cancel(); stop(); _ = server.Close(); client.CloseIdleConnections() }
	if ctx.Err() != nil {
		closeProxy()
		return "", nil, errors.New("media proxy cancelled")
	}
	return "http://" + listener.Addr().String(), closeProxy, nil
}
