package irc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lrstanley/girc"
	ergoapi "ircord/backend/internal/ergo"
)

// ModerationLogin names the operator entry on the associated Ergo server that the
// backend uses when a server owner deletes a message they could not delete
// themselves. The operator class needs only the history capability (and
// nofakelag for bulk deletions). Without it, server-owner deletion is off.
type ModerationLogin struct {
	Operator string
	Password string
}

var (
	// ErrModerationUnavailable reports that the moderation login is not configured
	// or could not reach operator status on the associated server.
	ErrModerationUnavailable = errors.New("server-owner deletion is not available on this server")
)

// moderator holds one IRC connection on the associated server, logged in to a
// backend-owned account and promoted with OPER. It is connected on first use and
// sends one REDACT at a time.
type moderator struct {
	m     *Manager
	login ModerationLogin

	mu         sync.Mutex // serializes connection setup and each REDACT
	registered bool
	failed     error // the last login failure, reported until moderationRetry passes
	failedAt   time.Time
	client     *girc.Client
	done       chan struct{}

	state   sync.Mutex
	opered  chan error
	ready   bool
	waits   map[string]chan string
	batches map[string]string
}

func newModerator(m *Manager) *moderator {
	o := m.options
	if o.Moderation == nil || o.Moderation.Operator == "" || o.Moderation.Password == "" || o.AssociatedServer == nil || o.Home.ID == "" {
		return nil
	}
	return &moderator{m: m, login: *o.Moderation, waits: map[string]chan string{}, batches: map[string]string{}}
}

// identity derives the moderation account from the operator credential, so no
// extra secret is stored and the account name cannot be guessed and claimed by
// someone else first.
func (mod *moderator) identity() (account, password string) {
	sum := func(purpose string) string {
		h := hmac.New(sha256.New, []byte(mod.login.Password))
		h.Write([]byte(purpose + "\x00" + mod.login.Operator))
		return hex.EncodeToString(h.Sum(nil))
	}
	return "ircordsvc" + sum("ircord moderation account")[:12], sum("ircord moderation password")
}

// moderationRetry is how long a failed login is reported again without a new
// attempt, so repeated deletions don't log in to the IRC server over and over.
var moderationRetry = 30 * time.Second

// connect returns a client that has completed OPER, or the recent login
// failure. Callers hold mod.mu.
func (mod *moderator) connect(ctx context.Context) (*girc.Client, error) {
	if mod.failed != nil && time.Since(mod.failedAt) < moderationRetry {
		return nil, mod.failed
	}
	client, err := mod.dial(ctx)
	mod.failed, mod.failedAt = nil, time.Time{}
	var service *ServiceError
	if errors.As(err, &service) {
		mod.failed, mod.failedAt = err, time.Now()
	}
	return client, err
}

// dial reuses a working connection or logs in again. Callers hold mod.mu.
func (mod *moderator) dial(ctx context.Context) (*girc.Client, error) {
	mod.state.Lock()
	if mod.client != nil && mod.ready && mod.client.IsConnected() {
		client := mod.client
		mod.state.Unlock()
		return client, nil
	}
	mod.state.Unlock()
	mod.close()
	account, password := mod.identity()
	server := mod.m.options.AssociatedServer
	if !mod.registered {
		err := server.Register(ctx, account, password)
		if errors.Is(err, ergoapi.ErrAccountExists) {
			var matches bool
			matches, err = server.CheckPassword(ctx, account, password)
			if err == nil && !matches {
				return nil, serviceError(503, "The moderation account exists with a different password.")
			}
		}
		if err != nil {
			return nil, serviceError(503, "The moderation account could not be set up on the IRC server.")
		}
		mod.registered = true
	}
	home := mod.m.options.Home
	caps := map[string][]string{"labeled-response": nil, "message-tags": nil, "batch": nil, "draft/message-redaction": nil}
	cfg := girc.Config{Server: home.Host, Port: home.Port, Nick: account, User: "ircord", Name: "IRCord moderation", Version: ctcpVersion, SSL: home.TLS, TLSConfig: mod.m.options.TLSConfig, DisableSTS: true, AllowFlood: true, SupportedCaps: caps, SASL: &girc.SASLPlain{User: account, Pass: password}}
	if cfg.TLSConfig != nil {
		cfg.TLSConfig = cfg.TLSConfig.Clone()
		if cfg.TLSConfig.ServerName == "" {
			cfg.TLSConfig.ServerName = home.Host
		}
	}
	client := girc.New(cfg)
	opered := make(chan error, 1)
	report := func(err error) {
		select {
		case opered <- err:
		default:
		}
	}
	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		c.Send(&girc.Event{Command: "OPER", Params: []string{mod.login.Operator, mod.login.Password}, Sensitive: true})
	})
	client.Handlers.Add("381", func(_ *girc.Client, _ girc.Event) { report(nil) })
	for _, refused := range []string{"464", "491", "481"} {
		client.Handlers.Add(refused, func(_ *girc.Client, _ girc.Event) {
			report(serviceError(503, "The IRC server refused the moderation operator login."))
		})
	}
	for _, failed := range []string{"902", "904", "905"} {
		client.Handlers.Add(failed, func(_ *girc.Client, _ girc.Event) {
			report(serviceError(503, "The moderation account could not log in to the IRC server."))
		})
	}
	client.Handlers.Add(girc.ALL_EVENTS, func(_ *girc.Client, e girc.Event) { mod.receive(e) })
	done := make(chan struct{})
	mod.state.Lock()
	mod.client, mod.done, mod.ready, mod.opered = client, done, false, opered
	mod.state.Unlock()
	go func() {
		defer close(done)
		_ = client.Connect()
		report(serviceError(503, "The moderation connection to the IRC server failed."))
		mod.state.Lock()
		if mod.client == client {
			mod.ready = false
		}
		mod.state.Unlock()
		mod.fail()
	}()
	ctx, cancel := context.WithTimeout(ctx, mod.m.options.ConnectTimeout)
	defer cancel()
	select {
	case err := <-opered:
		if err != nil {
			mod.close()
			return nil, err
		}
	case <-ctx.Done():
		mod.close()
		return nil, serviceError(503, "The moderation connection to the IRC server timed out.")
	}
	if !client.HasCapability("labeled-response") {
		mod.close()
		return nil, serviceError(501, "The IRC server lacks labeled-response, which server-owner deletion needs.")
	}
	mod.state.Lock()
	mod.ready = true
	mod.state.Unlock()
	return client, nil
}

// receive completes a waiting REDACT from its labeled answer. A FAIL or error
// numeric carries the refusal; anything else confirms it.
func (mod *moderator) receive(e girc.Event) {
	mod.state.Lock()
	defer mod.state.Unlock()
	if e.Command == "BATCH" && len(e.Params) > 0 {
		ref := e.Params[0]
		if strings.HasPrefix(ref, "+") && len(e.Params) > 1 && e.Params[1] == "labeled-response" && e.Tags["label"] != "" {
			mod.batches[ref[1:]] = e.Tags["label"]
		} else if strings.HasPrefix(ref, "-") {
			if label := mod.batches[ref[1:]]; label != "" {
				delete(mod.batches, ref[1:])
				mod.finish(label, "")
			}
		}
		return
	}
	label, inBatch := e.Tags["label"], false
	if label == "" {
		label, inBatch = mod.batches[e.Tags["batch"]], true
	}
	if label == "" {
		return
	}
	refused := e.Command == "FAIL" || len(e.Command) == 3 && e.Command[0] == '4'
	switch {
	case refused:
		reason := strings.TrimSpace(e.Last())
		if reason == "" {
			reason = "refused"
		}
		mod.finish(label, reason)
	case !inBatch:
		mod.finish(label, "")
	}
}

// finish requires mod.state.
func (mod *moderator) finish(label, reason string) {
	if wait := mod.waits[label]; wait != nil {
		delete(mod.waits, label)
		wait <- reason
	}
}

// fail ends every waiting request after the connection closed.
func (mod *moderator) fail() {
	mod.state.Lock()
	defer mod.state.Unlock()
	for label, wait := range mod.waits {
		delete(mod.waits, label)
		close(wait)
	}
	mod.batches = map[string]string{}
}

func (mod *moderator) close() {
	mod.state.Lock()
	client, done := mod.client, mod.done
	mod.client, mod.done, mod.ready = nil, nil, false
	mod.state.Unlock()
	if client != nil {
		client.Close()
		<-done
	}
}

// redact deletes one message through the moderation login and waits for the
// server's labeled answer.
func (mod *moderator) redact(ctx context.Context, target, messageID, reason string) error {
	mod.mu.Lock()
	defer mod.mu.Unlock()
	client, err := mod.connect(ctx)
	if err != nil {
		return err
	}
	label := "moderate:" + id()
	wait := make(chan string, 1)
	mod.state.Lock()
	mod.waits[label] = wait
	mod.state.Unlock()
	params := []string{target, messageID}
	if reason = strings.TrimSpace(reason); reason != "" {
		params = append(params, reason)
	}
	client.Send(&girc.Event{Command: "REDACT", Params: params, Tags: girc.Tags{"label": label}})
	timer := time.NewTimer(RedactTimeout)
	defer timer.Stop()
	select {
	case refusal, ok := <-wait:
		if !ok {
			return ErrDisconnected
		}
		if refusal != "" {
			return refusedError(refusal)
		}
		return nil
	case <-timer.C:
	case <-ctx.Done():
	}
	mod.state.Lock()
	delete(mod.waits, label)
	mod.state.Unlock()
	return ErrRedactUnconfirmed
}

// moderationNetwork returns the network ID on which server owners can delete any
// channel message, or "" when the moderation login is not configured.
func (m *Manager) moderationNetwork() string {
	if m.moderator == nil {
		return ""
	}
	return m.options.Home.ID
}

// ModerationCapability reports where owner can delete other people's channel
// messages as a server owner, or nil when they can't anywhere.
func (m *Manager) ModerationCapability(owner string) *ModerationCapability {
	network := m.moderationNetwork()
	if network == "" || !m.isServerOwner(owner) {
		return nil
	}
	return &ModerationCapability{NetworkID: network}
}
