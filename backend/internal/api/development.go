package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// DevelopmentSessions preserves the frontend demo's opaque-cookie boundary.
// It is deliberately replaceable and provides no login or durable accounts.
func DevelopmentSessions() Identity {
	var mu sync.Mutex
	sessions := map[string]time.Time{}
	return func(w http.ResponseWriter, r *http.Request) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		if cookie, err := r.Cookie("ircord_dev"); err == nil {
			if expires, ok := sessions[cookie.Value]; ok && now.Before(expires) {
				sessions[cookie.Value] = now.Add(24 * time.Hour)
				digest := sha256.Sum256([]byte(cookie.Value))
				return hex.EncodeToString(digest[:]), nil
			}
		}
		for token, expires := range sessions {
			if now.After(expires) {
				delete(sessions, token)
			}
		}
		if len(sessions) >= 500 {
			return "", fmt.Errorf("development session capacity reached")
		}
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return "", err
		}
		token := hex.EncodeToString(raw[:])
		sessions[token] = now.Add(24 * time.Hour)
		http.SetCookie(w, &http.Cookie{Name: "ircord_dev", Value: token, Path: "/api", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: 86400})
		digest := sha256.Sum256([]byte(token))
		return hex.EncodeToString(digest[:]), nil
	}
}
