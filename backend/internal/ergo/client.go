package ergo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

var ErrNotFound = errors.New("requested server identity is not available")

type Client struct {
	server, token string
	base          *url.URL
	http          *http.Client
	slots         chan struct{}
}

func httpTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil // Never send the privileged credential through environment proxies.
	t.MaxConnsPerHost, t.MaxIdleConnsPerHost = 4, 4
	return t
}
func newHTTPClient(t *http.Transport, timeout time.Duration) *http.Client {
	return &http.Client{Transport: t, Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}
func (c *Client) Close() { c.http.CloseIdleConnections() }

func (c *Client) call(ctx context.Context, path string, input, output any) error {
	ctx, cancel := context.WithTimeout(ctx, c.http.Timeout)
	defer cancel()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return errors.New("associated IRC server request cancelled")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return errors.New("cannot construct associated IRC server request")
	}
	u := c.base.JoinPath(path)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return errors.New("cannot construct associated IRC server request")
	}
	r.Header.Set("Authorization", "Bearer "+c.token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	response, err := c.http.Do(r)
	if err != nil {
		return errors.New("associated IRC server API is unavailable; check connectivity, TLS, and configuration")
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return errors.New("associated IRC server API rejected its backend credential")
	}
	if response.StatusCode != 200 {
		return errors.New("associated IRC server API request failed (redirects are refused)")
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(b) > 2<<20 {
		return errors.New("associated IRC server response could not be read within its limits")
	}
	if json.Unmarshal(b, output) != nil {
		return errors.New("associated IRC server returned an invalid response")
	}
	return nil
}

// sanitize is applied at the vendor boundary, before DTOs reach any client or
// diagnostic output. Unknown vendor fields (email, IPs, etc.) are not forwarded.
func (c *Client) sanitize(value string) string {
	value = strings.ReplaceAll(value, c.token, "[redacted]")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, value)
}

func (c *Client) Status(ctx context.Context) ErgoStatus {
	status := ErgoStatus{Server: c.server, State: "unavailable", CheckedAt: time.Now().UTC()}
	var wire struct {
		Success           bool                                                      `json:"success"`
		Version           string                                                    `json:"version"`
		GoVersion         string                                                    `json:"go_version"`
		StartedAt         time.Time                                                 `json:"start_time"`
		Users             *struct{ Total, Invisible, Operators, Unknown, Max *int } `json:"users"`
		Channels, Servers *int
	}
	err := c.call(ctx, "v1/status", nil, &wire)
	if err == nil {
		valid := wire.Success && wire.Version != "" && wire.GoVersion != "" && !wire.StartedAt.IsZero() && !wire.StartedAt.After(time.Now().Add(time.Minute)) && wire.Users != nil
		if valid {
			for _, n := range []*int{wire.Users.Total, wire.Users.Invisible, wire.Users.Operators, wire.Users.Unknown, wire.Users.Max, wire.Channels, wire.Servers} {
				if n == nil || *n < 0 {
					valid = false
					break
				}
			}
		}
		if !valid {
			err = errors.New("associated IRC server returned invalid status statistics")
		}
	}
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.State = "ok"
	status.Statistics = &ErgoStatistics{Version: c.sanitize(wire.Version), GoVersion: c.sanitize(wire.GoVersion), StartedAt: wire.StartedAt,
		UptimeSeconds: max(0, int64(time.Since(wire.StartedAt).Seconds())),
		Users:         ErgoUsers{Total: *wire.Users.Total, Invisible: *wire.Users.Invisible, Operators: *wire.Users.Operators, Unknown: *wire.Users.Unknown, Peak: *wire.Users.Max}, Channels: *wire.Channels, Servers: *wire.Servers}
	return status
}

func (c *Client) Channels(ctx context.Context) ([]ErgoChannel, error) {
	var wire struct {
		Success  bool
		Channels []struct {
			Name, Topic, Owner                     string
			UserCount                              int
			HasKey, InviteOnly, Secret, Registered bool
		}
	}
	if err := c.call(ctx, "v1/list", nil, &wire); err != nil {
		return nil, err
	}
	if !wire.Success || wire.Channels == nil {
		return nil, errors.New("associated IRC server could not list channels")
	}
	out := make([]ErgoChannel, 0, len(wire.Channels))
	for _, channel := range wire.Channels {
		if channel.Name == "" || channel.UserCount < 0 {
			return nil, errors.New("associated IRC server returned invalid channels")
		}
		out = append(out, ErgoChannel{Name: c.sanitize(channel.Name), Topic: c.sanitize(channel.Topic), Owner: c.sanitize(channel.Owner), Users: channel.UserCount, HasKey: channel.HasKey, InviteOnly: channel.InviteOnly, Secret: channel.Secret, Registered: channel.Registered})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) User(ctx context.Context, nick string) (ErgoUser, error) {
	var wire struct {
		Success, Present                         bool
		Nickname, Account, Realname, Modes, Away string
		Sessions                                 int `json:"session_count"`
		Channels                                 []ErgoUserChannel
	}
	if err := c.call(ctx, "v1/whois", map[string]string{"nickname": nick}, &wire); err != nil {
		return ErgoUser{}, err
	}
	if !wire.Success {
		return ErgoUser{}, errors.New("associated IRC server could not inspect nickname")
	}
	if !wire.Present {
		return ErgoUser{}, ErrNotFound
	}
	if wire.Nickname == "" || wire.Sessions < 0 {
		return ErgoUser{}, errors.New("associated IRC server returned invalid nickname data")
	}
	channels := make([]ErgoUserChannel, 0, len(wire.Channels))
	for _, channel := range wire.Channels {
		channels = append(channels, ErgoUserChannel{Name: c.sanitize(channel.Name), Mode: c.sanitize(channel.Mode)})
	}
	return ErgoUser{Nickname: c.sanitize(wire.Nickname), Account: c.sanitize(wire.Account), Realname: c.sanitize(wire.Realname), Modes: c.sanitize(wire.Modes), Away: c.sanitize(wire.Away), Sessions: wire.Sessions, Channels: channels}, nil
}

type wireAccount struct {
	Success      bool
	AccountName  string
	RegisteredAt *time.Time
	Channels     []string
}

func (c *Client) account(w wireAccount) ErgoAccount {
	a := ErgoAccount{Name: c.sanitize(w.AccountName), RegisteredAt: w.RegisteredAt, Channels: make([]string, 0, len(w.Channels))}
	for _, channel := range w.Channels {
		a.Channels = append(a.Channels, c.sanitize(channel))
	}
	return a
}
func (c *Client) Accounts(ctx context.Context) ([]ErgoAccount, error) {
	var wire struct {
		Success  bool
		Accounts []wireAccount
	}
	if err := c.call(ctx, "v1/ns/list", nil, &wire); err != nil {
		return nil, err
	}
	if !wire.Success || wire.Accounts == nil {
		return nil, errors.New("associated IRC server could not list accounts")
	}
	out := make([]ErgoAccount, 0, len(wire.Accounts))
	for _, account := range wire.Accounts {
		if !account.Success || account.AccountName == "" {
			return nil, errors.New("associated IRC server returned invalid accounts")
		}
		out = append(out, c.account(account))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (c *Client) Account(ctx context.Context, name string) (ErgoAccount, error) {
	var wire wireAccount
	if err := c.call(ctx, "v1/ns/info", map[string]string{"accountName": name}, &wire); err != nil {
		return ErgoAccount{}, err
	}
	if !wire.Success {
		return ErgoAccount{}, ErrNotFound
	}
	if wire.AccountName == "" {
		return ErgoAccount{}, errors.New("associated IRC server returned invalid account data")
	}
	return c.account(wire), nil
}

var ErrAccountExists = errors.New("that account already exists on the IRC server")
var ErrInvalidPassphrase = errors.New("the IRC server did not accept that password")

// ErrAccountRefused is Ergo refusing to create an account without saying why (UNKNOWN_ERROR);
// in practice a name it doesn't allow.
var ErrAccountRefused = errors.New("the IRC server did not accept that account name")

type accountResult struct {
	Success   bool
	ErrorCode string `json:"errorCode"`
}

// Server is the associated IRC endpoint (host:port) users connect to.
func (c *Client) Server() string { return c.server }

// Register creates an account on the associated server, as NS SAREGISTER does. The passphrase
// is only sent to Ergo; it never appears in errors.
func (c *Client) Register(ctx context.Context, account, passphrase string) error {
	var wire accountResult
	if err := c.call(ctx, "v1/ns/saregister", map[string]string{"accountName": account, "passphrase": passphrase}, &wire); err != nil {
		return err
	}
	switch {
	case wire.Success:
		return nil
	case wire.ErrorCode == "ACCOUNT_EXISTS":
		return ErrAccountExists
	case wire.ErrorCode == "INVALID_PASSPHRASE":
		return ErrInvalidPassphrase
	case wire.ErrorCode == "UNKNOWN_ERROR":
		return ErrAccountRefused
	}
	return errors.New("associated IRC server could not register the account")
}

// SetPassword replaces an account's password on the associated server.
func (c *Client) SetPassword(ctx context.Context, account, passphrase string) error {
	var wire accountResult
	if err := c.call(ctx, "v1/ns/passwd", map[string]string{"accountName": account, "passphrase": passphrase}, &wire); err != nil {
		return err
	}
	switch {
	case wire.Success:
		return nil
	case wire.ErrorCode == "ACCOUNT_DOES_NOT_EXIST":
		return ErrNotFound
	case wire.ErrorCode == "INVALID_PASSPHRASE":
		return ErrInvalidPassphrase
	}
	return errors.New("associated IRC server could not change the password")
}

// AccountExists reports whether the associated server has an account with this name.
func (c *Client) AccountExists(ctx context.Context, name string) (bool, error) {
	_, err := c.Account(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
