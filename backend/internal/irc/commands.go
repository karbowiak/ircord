package irc

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lrstanley/girc"
)

func rawEvent(line string) (*girc.Event, error) {
	if !clean(line, 510) || strings.TrimSpace(line) == "" || line[0] == ':' || line[0] == '@' {
		return nil, fmt.Errorf("invalid IRC line (maximum 510 UTF-8 bytes)")
	}
	event := girc.ParseEvent(line)
	if event == nil || event.Command == "" {
		return nil, fmt.Errorf("invalid IRC command")
	}
	event.Command = strings.ToUpper(event.Command)
	for _, r := range event.Command {
		if r < 'A' || r > 'Z' {
			return nil, fmt.Errorf("IRC command must contain letters")
		}
	}
	if slices.Contains([]string{"QUIT", "USER", "PASS", "CAP", "AUTHENTICATE", "WEBIRC", "STARTTLS", "BATCH"}, event.Command) {
		return nil, fmt.Errorf("this command is managed by IRCord")
	}
	if event.Command == "NICK" {
		if len(event.Params) != 1 {
			return nil, fmt.Errorf("NICK takes one nickname without spaces")
		}
		if err := validateNickname(event.Params[0], 0); err != nil {
			return nil, err
		}
	}
	return event, nil
}

func (m *Manager) Raw(owner, network, line string) error {
	return m.RawCommand(owner, network, id(), line)
}

func (m *Manager) RawCommand(owner, network, clientID, line string) error {
	if !token(clientID, 100) {
		return fmt.Errorf("invalid clientId")
	}
	event, err := rawEvent(line)
	if err != nil {
		return err
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return err
	}
	if err = c.denyPartyCommand(event); err != nil {
		return err
	}
	if event.Command == "AWAY" {
		reason := ""
		if len(event.Params) > 0 {
			reason = event.Last()
		}
		m.manualAway(owner, network, reason)
	}
	if event.Command == "NICK" {
		c.owner.mu.Lock()
		maximum := c.nicknameLimit()
		c.owner.mu.Unlock()
		if err := validateNickname(event.Params[0], maximum); err != nil {
			return err
		}
	}
	if event.Command == "SETNAME" && !c.has("setname") || event.Command == "MARKREAD" && !c.hasAll("draft/read-marker", "server-time") || event.Command == "CHATHISTORY" && !c.hasAll("draft/chathistory", "batch", "server-time") {
		return ErrUnsupported
	}
	if err := c.validateChannelCommand(event); err != nil {
		return err
	}
	if event.Command == "PRIVMSG" || event.Command == "NOTICE" {
		c.mu.Lock()
		limit := c.lineBytes
		c.mu.Unlock()
		if limit <= 0 {
			limit = safeMessageLineBytes(girc.DefaultMaxLineLength - girc.DefaultMaxPrefixLength)
		}
		if event.LenOpts(false) > limit {
			return fmt.Errorf("command exceeds this network's line limit; use the message composer")
		}
	}
	if c.has("labeled-response") {
		event.Tags = girc.Tags{"label": "raw:" + clientID}
	} else if event.Command == "CHATHISTORY" {
		return c.enqueueRawHistory(event, clientID)
	}
	if err := c.enqueue(event); err != nil {
		return err
	}
	if event.Command == "PRIVMSG" {
		m.endBriefAway(owner, network, c)
	}
	return nil
}

func (c *connection) receiveCommandFailure(e girc.Event) {
	rawClientID, raw := c.rawHistoryFailure(e)
	consumed := !raw && c.historyFailure(e)
	label := e.Tags["label"]
	c.mu.Lock()
	if label == "" {
		label = c.batchLabels[e.Tags["batch"]]
	}
	_, pending := c.pending[label]
	if pending {
		delete(c.pending, label)
	}
	c.mu.Unlock()
	reason := c.redactSecrets(e.Last())
	if c.redactionResult(label, reason) {
		return
	}
	if consumed {
		return
	}
	w := c.owner
	w.mu.Lock()
	if raw {
		w.sendCommandError(rawClientID, reason)
	} else if strings.HasPrefix(label, "raw:") {
		w.sendCommandError(strings.TrimPrefix(label, "raw:"), reason)
	} else {
		w.commandError(label, reason)
	}
	w.mu.Unlock()
}
