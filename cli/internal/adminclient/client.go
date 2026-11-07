// Package adminclient is a bounded, typed HTTP client with no backend lifecycle
// dependencies. Future administration commands share this transport boundary.
package adminclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"ircord/cli/internal/contract"
	"ircord/cli/internal/credentials"
)

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

// APIError deliberately excludes arbitrary upstream problem bodies and headers:
// a proxy or backend must not be able to reflect credentials into diagnostics.
type APIError struct{ Status int }

func (e *APIError) Error() string {
	switch e.Status {
	case 401, 403:
		return "admin authentication rejected; check the configured admin token and server"
	case 404:
		return "administration endpoint not found; check the backend URL and version"
	case 429:
		return "administration request rate limited; try again later"
	case 503:
		return "administration unavailable; check backend health and admin-token configuration"
	}
	if e.Status >= 300 && e.Status < 400 {
		return "administration redirect refused; use the backend's final URL"
	}
	return fmt.Sprintf("administration request failed (HTTP %d)", e.Status)
}

func New(server, token string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(server)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" {
		return nil, errors.New("server must be an HTTP(S) URL without credentials, query, fragment, or encoded path")
	}
	loopback := strings.EqualFold(u.Hostname(), "localhost")
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil {
		loopback = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return nil, errors.New("server must use HTTPS; HTTP is allowed only for loopback development")
	}
	if err := credentials.ValidateToken(token); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, errors.New("request timeout must be positive")
	}
	// Generated request constructors resolve paths relative to a directory URL.
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &Client{base: u, token: token, http: &http.Client{
		Timeout: timeout, Transport: transport,
		// Reject even same-origin redirects so configuration mistakes are visible.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

func (c *Client) Status(ctx context.Context) (contract.Status, error) {
	var status contract.Status
	// Generate both DTOs and request construction from Huma's published contract.
	r, err := contract.NewAdminStatusRequest(c.base.String())
	if err != nil {
		return status, errors.New("cannot construct administration request")
	}
	r = r.WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+c.token)
	r.Header.Set("Accept", "application/json")
	response, err := c.http.Do(r)
	if err != nil {
		if ctx.Err() != nil {
			return status, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return status, fmt.Errorf("administration request timed out: %w", context.DeadlineExceeded)
		}
		return status, errors.New("cannot reach backend; check the server URL, network, and TLS certificate")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return status, &APIError{Status: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		if ctx.Err() != nil {
			return status, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return status, fmt.Errorf("administration response timed out: %w", context.DeadlineExceeded)
		}
		return status, errors.New("cannot read administration response")
	}
	if len(body) > 1<<20 || json.Unmarshal(body, &status) != nil ||
		!status.Status.Valid() || !status.Database.Valid() ||
		status.Version == "" || status.Commit == "" || status.GoVersion == "" || status.StartedAt.IsZero() || status.UptimeSeconds < 0 {
		return contract.Status{}, errors.New("invalid administration status response; check backend compatibility")
	}
	status.Version = strings.ReplaceAll(status.Version, c.token, "[redacted]")
	status.Commit = strings.ReplaceAll(status.Commit, c.token, "[redacted]")
	status.GoVersion = strings.ReplaceAll(status.GoVersion, c.token, "[redacted]")
	// Huma's optional schema URI is metadata, not needed in terminal output.
	status.Schema = nil
	return status, nil
}
