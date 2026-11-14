package irc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

// profileLookupTimeout bounds how long a profile waits for the server or the other client.
const profileLookupTimeout = 10 * time.Second

// ctcpVersion is how IRCord connections answer CTCP VERSION: the client's name, without the
// build, Go version or operating system.
const ctcpVersion = "IRCord"

// profileConnection is the ready connection a profile lookup on networkID uses.
func (m *Manager) profileConnection(owner, networkID, nick string) (*connection, *girc.Client, func(string) string, error) {
	nick = strings.TrimSpace(nick)
	if owner == "" || nick == "" || utf8.RuneCountInString(nick) > 64 || !clean(nick, 256) ||
		strings.ContainsAny(nick, " ,*?!@\x01") || strings.HasPrefix(nick, "#") || strings.HasPrefix(nick, ":") {
		return nil, nil, nil, fmt.Errorf("enter a nickname")
	}
	w := m.space(owner)
	w.mu.Lock()
	n, ok := w.networks[networkID]
	c := w.connections[networkID]
	deleted := w.deleted
	w.mu.Unlock()
	if !ok || deleted {
		return nil, nil, nil, ErrNotFound
	}
	if c == nil {
		return nil, nil, nil, ErrDisconnected
	}
	c.mu.Lock()
	client, ready := c.client, c.ready
	c.mu.Unlock()
	if !ready || client == nil {
		return nil, nil, nil, ErrDisconnected
	}
	mapping := n.ISupport["CASEMAPPING"]
	return c, client, func(v string) string { return model.FoldIdentity(v, mapping) }, nil
}

// Whois asks the network about one nickname: who they are, where they connect from, which
// channels they show in and how long they have been idle. It is silent to the person asked.
func (m *Manager) Whois(ctx context.Context, owner, networkID, nick string) (model.WhoisDetails, error) {
	nick = strings.TrimSpace(nick)
	c, client, fold, err := m.profileConnection(owner, networkID, nick)
	if err != nil {
		return model.WhoisDetails{}, err
	}
	// Shared with other checks, so profile lookups can't crowd out the connection.
	select {
	case m.checkSlots <- struct{}{}:
		defer func() { <-m.checkSlots }()
	default:
		return model.WhoisDetails{}, ErrBusy
	}
	ctx, cancel := context.WithTimeout(ctx, profileLookupTimeout)
	defer cancel()
	var mu sync.Mutex
	d := model.WhoisDetails{Nickname: nick, Channels: []string{}}
	done := make(chan struct{}, 1)
	handler := client.Handlers.Add(girc.ALL_EVENTS, func(_ *girc.Client, e girc.Event) {
		if len(e.Params) < 2 || fold(e.Params[1]) != fold(nick) {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		applyWhoisLine(&d, e)
		if e.Command == "318" || e.Command == "401" {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	defer client.Handlers.Remove(handler)
	if err := c.enqueue(&girc.Event{Command: "WHOIS", Params: []string{nick}}); err != nil {
		return model.WhoisDetails{}, err
	}
	select {
	case <-ctx.Done():
		return model.WhoisDetails{}, ctx.Err()
	case <-c.ctx.Done():
		return model.WhoisDetails{}, ErrDisconnected
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		return d, nil
	}
}

// applyWhoisLine records one WHOIS reply line about the person.
func applyWhoisLine(d *model.WhoisDetails, e girc.Event) {
	switch e.Command {
	case "311": // RPL_WHOISUSER: <me> <nick> <user> <host> * :<real name>
		d.Found = true
		d.Nickname = e.Params[1]
		if len(e.Params) >= 4 {
			d.Username, d.Host = e.Params[2], e.Params[3]
		}
		d.RealName = e.Last()
	case "312": // RPL_WHOISSERVER: <me> <nick> <server> :<info>
		if len(e.Params) >= 3 {
			d.Server, d.ServerInfo = e.Params[2], e.Last()
		}
	case "313": // RPL_WHOISOPERATOR
		d.Operator = true
	case "317": // RPL_WHOISIDLE: <me> <nick> <idle> <signon> :seconds idle, signon time
		if len(e.Params) >= 3 {
			if idle, err := strconv.ParseInt(e.Params[2], 10, 64); err == nil && idle >= 0 {
				d.IdleSeconds = &idle
			}
		}
		if len(e.Params) >= 4 {
			if signon, err := strconv.ParseInt(e.Params[3], 10, 64); err == nil && signon > 0 {
				at := time.Unix(signon, 0).UTC()
				d.SignedOnAt = &at
			}
		}
	case "319": // RPL_WHOISCHANNELS: <me> <nick> :<channels>; may repeat
		for _, channel := range strings.Fields(e.Last()) {
			if len(d.Channels) < 100 {
				d.Channels = append(d.Channels, channel)
			}
		}
	case "330": // RPL_WHOISACCOUNT: <me> <nick> <account> :is logged in as
		if len(e.Params) >= 3 && e.Params[2] != "*" {
			d.Account = e.Params[2]
		}
	case "671": // RPL_WHOISSECURE
		d.Secure = true
	case "301": // RPL_AWAY: <me> <nick> :<message>
		d.Away = e.Last()
	}
}

// ctcpCommands are the queries a profile may send. Each one shows up in the other person's
// client, so they are sent only when asked for.
var ctcpCommands = map[string]bool{"VERSION": true, "TIME": true, "PING": true}

// CTCP sends one CTCP query to a nickname and waits for its reply. No reply in time is an
// answer too: many clients and bouncers ignore CTCP.
func (m *Manager) CTCP(ctx context.Context, owner, networkID, nick, command string) (model.CTCPReply, error) {
	command = strings.ToUpper(strings.TrimSpace(command))
	reply := model.CTCPReply{Command: command}
	if !ctcpCommands[command] {
		return reply, fmt.Errorf("command must be VERSION, TIME or PING")
	}
	nick = strings.TrimSpace(nick)
	c, client, fold, err := m.profileConnection(owner, networkID, nick)
	if err != nil {
		return reply, err
	}
	select {
	case m.checkSlots <- struct{}{}:
		defer func() { <-m.checkSlots }()
	default:
		return reply, ErrBusy
	}
	ctx, cancel := context.WithTimeout(ctx, profileLookupTimeout)
	defer cancel()
	token := strconv.FormatInt(time.Now().UnixNano(), 10)
	text := ""
	if command == "PING" {
		text = token
	}
	answers := make(chan string, 1)
	handler := client.Handlers.Add(girc.ALL_EVENTS, func(_ *girc.Client, e girc.Event) {
		ctcp := girc.DecodeCTCP(&e)
		if ctcp == nil || !ctcp.Reply || ctcp.Source == nil || fold(ctcp.Source.Name) != fold(nick) ||
			ctcp.Command != command || (command == "PING" && strings.TrimSpace(ctcp.Text) != token) {
			return
		}
		select {
		case answers <- ctcp.Text:
		default:
		}
	})
	defer client.Handlers.Remove(handler)
	sent := time.Now()
	if err := c.enqueue(&girc.Event{Command: girc.PRIVMSG, Params: []string{nick, girc.EncodeCTCPRaw(command, text)}}); err != nil {
		return reply, err
	}
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return reply, nil
		}
		return reply, ctx.Err()
	case <-c.ctx.Done():
		return reply, ErrDisconnected
	case answer := <-answers:
		reply.Answered = true
		if command == "PING" {
			latency := time.Since(sent).Milliseconds()
			reply.LatencyMs = &latency
		} else {
			reply.Reply = strings.TrimSpace(strings.TrimPrefix(answer, ":"))
		}
		return reply, nil
	}
}
