package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"ircord/backend/internal/admin"
	"ircord/backend/internal/auth"
)

type ownerKey struct{}

func owner(ctx context.Context) string { value, _ := ctx.Value(ownerKey{}).(string); return value }

func (s *Server) originAllowed(r *http.Request) bool {
	raw := r.Header.Get("Origin")
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if raw == scheme+"://"+r.Host {
		return true
	}
	for _, allowed := range s.options.Origins {
		if raw == allowed {
			return true
		}
	}
	return false
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		if !s.originAllowed(r) {
			writeProblem(w, "origin rejected", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if admin.IsPath(r.URL.Path) {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, httpState{w, r}))
		// Admin credentials never become an account owner or fall back to a cookie.
		// Keep the shared origin check above this boundary for browser callers.
		if admin.IsPath(r.URL.Path) {
			if s.options.Admin == nil {
				writeProblem(w, "administration is not configured", http.StatusServiceUnavailable)
				return
			}
			if !s.options.Admin.Authenticate(r) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="ircord-admin"`)
				writeProblem(w, "admin authentication required", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if s.options.Auth != nil && (r.URL.Path == "/api/v1/auth/signup" || r.URL.Path == "/api/v1/auth/login" || (r.Method == "GET" || r.Method == "HEAD") && (r.URL.Path == "/api/v1/auth/registration" || r.URL.Path == "/api/v1/auth/username")) {
			next.ServeHTTP(w, r)
			return
		}
		var identity string
		var err error
		if s.options.Auth != nil {
			var p auth.Principal
			p, err = s.options.Auth.Authenticate(r)
			if err == nil {
				identity = p.Account.ID
				r = r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
			}
		} else {
			identity, err = s.options.Identity(w, r)
		}
		if err != nil && !errors.Is(err, auth.ErrSession) {
			writeProblem(w, "authentication storage unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/api/v1/auth/session" && s.options.Auth != nil && identity == "" {
			next.ServeHTTP(w, r)
			return
		}
		if err != nil || identity == "" {
			writeProblem(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			if err := s.Manager.StorageError(identity); err != nil {
				writeProblem(w, "workspace storage unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ownerKey{}, identity)))
	})
}
