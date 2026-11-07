// Package adminclient is the bounded, typed HTTP boundary for remote administration.
package adminclient

import (
	"bytes"
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
type APIError struct{ Status int }

func (e *APIError) Error() string {
	switch e.Status {
	case 401, 403:
		return "admin authentication rejected; check the configured admin token and server"
	case 404:
		return "administration resource not found or not offered by this backend"
	case 429:
		return "administration request rate limited; try again later"
	case 503:
		return "administration unavailable; check backend health and integration configuration"
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
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &Client{base: u, token: token, http: &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Close() { c.http.CloseIdleConnections() }
func (c *Client) get(ctx context.Context, build func(string) (*http.Request, error), out any) error {
	r, err := build(c.base.String())
	if err != nil {
		return errors.New("cannot construct administration request")
	}
	r = r.WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+c.token)
	r.Header.Set("Accept", "application/json")
	response, err := c.http.Do(r)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("administration request timed out: %w", context.DeadlineExceeded)
		}
		return errors.New("cannot reach backend; check the server URL, network, and TLS certificate")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return &APIError{Status: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("administration response timed out: %w", context.DeadlineExceeded)
		}
		return errors.New("cannot read administration response")
	}
	if len(body) > 2<<20 || json.Unmarshal(body, out) != nil {
		return errors.New("invalid administration response; check backend compatibility")
	}
	// Sanitize all typed string fields, including optional integration responses.
	encoded, err := json.Marshal(out)
	if err != nil {
		return errors.New("invalid administration response")
	}
	encoded = bytes.ReplaceAll(encoded, []byte(c.token), []byte("[redacted]"))
	if json.Unmarshal(encoded, out) != nil {
		return errors.New("invalid administration response")
	}
	return nil
}
func (c *Client) Status(ctx context.Context) (contract.Status, error) {
	var out contract.Status
	if err := c.get(ctx, contract.NewAdminStatusRequest, &out); err != nil {
		return out, err
	}
	if !out.Status.Valid() || !out.Database.Valid() || out.Version == "" || out.Commit == "" || out.GoVersion == "" || out.StartedAt.IsZero() || out.UptimeSeconds < 0 {
		return contract.Status{}, errors.New("invalid administration status response; check backend compatibility")
	}
	out.Schema = nil
	return out, nil
}
func (c *Client) Capabilities(ctx context.Context) (contract.AdminCapabilities, error) {
	var out contract.AdminCapabilities
	err := c.get(ctx, contract.NewAdminCapabilitiesRequest, &out)
	if err == nil && out.Features == nil {
		err = errors.New("invalid administration capability response")
	}
	return out, err
}
func (c *Client) Doctor(ctx context.Context) (contract.Doctor, error) {
	var out contract.Doctor
	err := c.get(ctx, contract.NewAdminDoctorRequest, &out)
	if err == nil && (!out.Status.Valid() || out.Checks == nil) {
		err = errors.New("invalid administration diagnostic response")
	}
	out.Schema = nil
	return out, err
}
func (c *Client) ErgoStatus(ctx context.Context) (contract.ErgoStatus, error) {
	var out contract.ErgoStatus
	err := c.get(ctx, contract.NewAdminErgoStatusRequest, &out)
	if err == nil && (!out.State.Valid() || out.Server == "" || out.CheckedAt.IsZero() || (out.State == "ok" && out.Statistics == nil) || (out.State == "unavailable" && out.Statistics != nil)) {
		err = errors.New("invalid associated-server status response")
	}
	out.Schema = nil
	return out, err
}
func (c *Client) ErgoChannels(ctx context.Context) (contract.ItemsErgoChannel, error) {
	var out contract.ItemsErgoChannel
	err := c.get(ctx, contract.NewAdminErgoChannelsRequest, &out)
	if err == nil && out.Items == nil {
		err = errors.New("invalid associated-server channel response")
	}
	out.Schema = nil
	return out, err
}
func (c *Client) ErgoUser(ctx context.Context, name string) (contract.ErgoUser, error) {
	var out contract.ErgoUser
	err := c.get(ctx, func(server string) (*http.Request, error) { return contract.NewAdminErgoUserRequest(server, name) }, &out)
	if err == nil && (out.Nickname == "" || out.Sessions < 0 || out.Channels == nil) {
		err = errors.New("invalid associated-server nickname response")
	}
	out.Schema = nil
	return out, err
}
func (c *Client) ErgoAccounts(ctx context.Context) (contract.ItemsErgoAccount, error) {
	var out contract.ItemsErgoAccount
	err := c.get(ctx, contract.NewAdminErgoAccountsRequest, &out)
	if err == nil && out.Items == nil {
		err = errors.New("invalid associated-server account response")
	}
	out.Schema = nil
	return out, err
}
func (c *Client) ErgoAccount(ctx context.Context, name string) (contract.ErgoAccount, error) {
	var out contract.ErgoAccount
	err := c.get(ctx, func(server string) (*http.Request, error) { return contract.NewAdminErgoAccountRequest(server, name) }, &out)
	if err == nil && (out.Name == "" || out.Channels == nil) {
		err = errors.New("invalid associated-server account response")
	}
	out.Schema = nil
	return out, err
}
