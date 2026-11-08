package irc

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

type ServerLine = model.ServerLine
type ServerLogPage = model.ServerLogPage
type ConnectionInfo = model.ConnectionInfo

func newConnectionInfo(network string) ConnectionInfo {
	return ConnectionInfo{NetworkID: network, State: "disconnected", ISupport: map[string]string{}, OfferedCapabilities: map[string]string{}, NegotiatedCapabilities: []string{}, MOTD: []string{}}
}
func (c *connection) redactSecrets(text string) string {
	secrets := []string{c.config.SASLPassword, c.config.ServerPassword}
	if c.config.Auth != nil && c.config.Auth.Password != nil {
		secrets = append(secrets, *c.config.Auth.Password)
	}
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	return text
}
func (c *connection) capture(e girc.Event, outgoing bool) {
	if strings.HasPrefix(e.Command, "CLIENT_") {
		return
	}
	params := append([]string{}, e.Params...)
	for i, value := range params {
		params[i] = c.redactSecrets(value)
	}
	if outgoing && e.Command == "JOIN" && len(params) > 1 {
		params[1] = "[redacted]"
	}
	if e.Command == "AUTHENTICATE" || e.Command == "PASS" || e.Command == "OPER" || e.Command == "WEBIRC" {
		params = []string{"[redacted]"}
	}
	if e.Command == "PRIVMSG" && len(params) >= 2 && strings.EqualFold(params[0], "NickServ") {
		upper := strings.ToUpper(params[1])
		if strings.HasPrefix(upper, "IDENTIFY ") || strings.HasPrefix(upper, "REGISTER ") || strings.HasPrefix(upper, "SET PASSWORD ") {
			params[1] = "[redacted]"
		}
	}
	source := ""
	if e.Source != nil {
		source = c.redactSecrets(e.Source.String())
	}
	kind := "raw"
	switch {
	case outgoing:
		kind = "outgoing"
	case e.Command == "ERROR" || e.Command == "FAIL" || (len(e.Command) == 3 && e.Command[0] >= '4' && e.Command[0] <= '5'):
		kind = "error"
	case slices.Contains([]string{"372", "375", "376", "422"}, e.Command):
		kind = "motd"
	case len(e.Command) == 3 && e.Command[0] >= '0' && e.Command[0] <= '9':
		kind = "numeric"
	case e.Command == "NOTICE":
		kind = "notice"
	}
	text := e.Command
	if len(params) > 0 {
		text = params[len(params)-1]
	}
	tags := map[string]string{}
	for k, v := range e.Tags {
		tags[k] = c.redactSecrets(v)
	}
	line := ServerLine{Tags: tags, ID: id(), NetworkID: c.config.ID, Time: time.Now().UTC(), Kind: kind, Command: e.Command, Source: source, Params: params, Text: text}
	bulkList := e.Command == "322" || e.Command == "323"
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	w.trace[c.config.ID] = append(w.trace[c.config.ID], line)
	if len(w.trace[c.config.ID]) > 500 {
		copy(w.trace[c.config.ID], w.trace[c.config.ID][1:])
		w.trace[c.config.ID] = w.trace[c.config.ID][:500]
	}
	for ch := range w.traceSubscribers[c.config.ID] {
		select {
		case ch <- clone(line):
		default:
			delete(w.traceSubscribers[c.config.ID], ch)
			close(ch)
		}
	}
	// The server window excludes conversation traffic. The optional raw trace
	// above retains the complete redacted receive stream, including those lines.
	status := outgoing || !(e.Command == "PRIVMSG" || e.Command == "TAGMSG" || e.Command == "JOIN" || e.Command == "PART" || e.Command == "PING" || e.Command == "PONG")
	historyStatus := e.Source != nil && strings.EqualFold(e.Source.Name, "HistServ") && strings.HasPrefix(e.Tags["msgid"], "_") && (e.Command == "PRIVMSG" || e.Command == "NOTICE")
	if historyStatus {
		status = true
	}
	if e.Command == "TAGMSG" && (e.Tags["+typing"] != "" || e.Tags["+draft/typing"] != "" || e.Tags["draft/typing"] != "") {
		// Activity is visible in the optional bounded raw trace, but never creates
		// durable server-window events or consumes the account replay cursor.
		status = false
	}
	if e.Command == "NOTICE" && e.Source != nil && e.Source.Ident != "" && !historyStatus {
		status = false
	}
	if status {
		if historyStatus {
			line.ID = c.config.ID + ":" + e.Tags["msgid"] + ":status"
			if parsed, err := time.Parse(time.RFC3339Nano, e.Tags["time"]); err == nil {
				line.Time = parsed
			}
			line.Kind = "notice"
			if len(params) > 0 {
				line.Text = "[" + params[0] + "] " + line.Text
			}
			if w.hasServerLine(line.ID, c.config.ID) {
				return
			}
		}
		w.serverLogs[c.config.ID] = append(w.serverLogs[c.config.ID], line)
		if len(w.serverLogs[c.config.ID]) > 500 {
			copy(w.serverLogs[c.config.ID], w.serverLogs[c.config.ID][1:])
			w.serverLogs[c.config.ID] = w.serverLogs[c.config.ID][:500]
		}
		if bulkList && e.Command == "323" {
			w.emit("server-log.updated", model.ServerLogBatch{NetworkID: c.config.ID, Lines: w.serverLogs[c.config.ID]})
		} else if !(bulkList && e.Command == "322") {
			w.emit("server.line", line)
		}
	}
	info := w.info[c.config.ID]
	changed := false
	switch e.Command {
	case "001":
		info.ServerName = source
		changed = true
	case "005":
		for _, token := range safeISupportParams(e.Params) {
			key, value, _ := strings.Cut(token, "=")
			if strings.HasPrefix(key, "-") {
				delete(info.ISupport, strings.TrimPrefix(key, "-"))
			} else {
				info.ISupport[key] = c.redactSecrets(value)
			}
		}
		info.FilehostURL = info.ISupport["draft/FILEHOST"]
		if info.FilehostURL == "" {
			info.FilehostURL = info.ISupport["soju.im/FILEHOST"]
		}
		changed = true
	case "CAP":
		if len(e.Params) >= 2 && (e.Params[1] == "LS" || e.Params[1] == "NEW" || e.Params[1] == "DEL") {
			for _, token := range strings.Fields(e.Last()) {
				key, value, _ := strings.Cut(token, "=")
				if e.Params[1] == "DEL" {
					delete(info.OfferedCapabilities, key)
				} else {
					info.OfferedCapabilities[key] = c.redactSecrets(value)
				}
			}
			changed = true
		}
	case "375":
		info.MOTD = []string{}
		changed = true
	case "372":
		info.MOTD = append(info.MOTD, c.redactSecrets(e.Last()))
		if len(info.MOTD) > 200 {
			info.MOTD = info.MOTD[len(info.MOTD)-200:]
		}
		changed = true
	case "ERROR", "FAIL":
		info.LastError = c.redactSecrets(e.Last())
		changed = true
	}
	if e.Command == "005" {
		n := w.networks[c.config.ID]
		n.ISupport = clone(info.ISupport)
		c.mu.Lock()
		n.Features = c.features(n.ISupport)
		c.mu.Unlock()
		w.networks[n.ID] = n
		w.emit("network.updated", n)
	}
	if changed {
		w.info[c.config.ID] = info
		w.emit("connection.updated", info)
	}
}
func (c *connection) connectionState(state string, client *girc.Client, connectionErr error) {
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	info := w.info[c.config.ID]
	info.State = state
	if state == "connecting" {
		n := w.networks[c.config.ID]
		n.Status = "connecting"
		n.StatusDetail = ""
		n.Features = model.Features{Metadata: []string{}}
		n.Capabilities = []string{}
		n.ISupport = map[string]string{}
		info = newConnectionInfo(c.config.ID)
		info.State = state
		w.networks[n.ID] = n
		w.emit("network.updated", n)
	}
	if connectionErr != nil {
		info.LastError = c.redactSecrets(connectionErr.Error())
	}
	if state == "connected" {
		now := time.Now().UTC()
		info.ConnectedAt = &now
		info.LastError = ""
		info.NegotiatedCapabilities = append([]string{}, w.networks[c.config.ID].Capabilities...)
		if client != nil {
			if value, err := client.TLSConnectionState(); err == nil {
				tlsInfo := &model.TLSInfo{Version: tls.VersionName(value.Version), Cipher: tls.CipherSuiteName(value.CipherSuite), Verified: len(value.VerifiedChains) > 0, ServerName: value.ServerName}
				if len(value.PeerCertificates) > 0 {
					sum := sha256.Sum256(value.PeerCertificates[0].Raw)
					tlsInfo.CertificateSHA256 = hex.EncodeToString(sum[:])
				}
				info.TLS = tlsInfo
			}
		}
	}
	w.info[c.config.ID] = info
	w.emit("connection.updated", info)
}
func (m *Manager) ConnectionInfo(owner, network string) (ConnectionInfo, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.networks[network]; !ok {
		return ConnectionInfo{}, ErrNotFound
	}
	info, ok := w.info[network]
	if !ok {
		info = newConnectionInfo(network)
	}
	return clone(info), nil
}
func (m *Manager) ServerLog(owner, network, before string, limit int, raw bool) (ServerLogPage, error) {
	if limit < 1 || limit > 100 {
		return ServerLogPage{}, fmt.Errorf("limit must be 1 to 100")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.networks[network]; !ok {
		return ServerLogPage{}, ErrNotFound
	}
	if !raw && w.store != nil {
		if !w.systemLogsBackfilled[network] {
			lines, err := w.store.SystemLogHistory(owner, network)
			if err != nil {
				return ServerLogPage{}, ErrStorage
			}
			if len(lines) > 0 {
				// Import retained notices in one transaction/invalidation so opening
				// Server cannot overflow a browser's durable live-event queue.
				w.emit("server-log.updated", model.ServerLogBatch{NetworkID: network, Lines: lines})
			}
			if w.storageErr != nil {
				return ServerLogPage{}, ErrStorage
			}
			if w.systemLogsBackfilled == nil {
				w.systemLogsBackfilled = map[string]bool{}
			}
			w.systemLogsBackfilled[network] = true
		}
		return w.store.ServerLog(owner, network, before, limit)
	}
	items := w.serverLogs[network]
	if raw {
		items = w.trace[network]
	}
	end := len(items)
	if before != "" {
		end = -1
		for i, v := range items {
			if v.ID == before {
				end = i
				break
			}
		}
		if end < 0 {
			return ServerLogPage{}, fmt.Errorf("invalid log cursor")
		}
	}
	start := max(0, end-limit)
	page := ServerLogPage{Items: clone(items[start:end])}
	if page.Items == nil {
		page.Items = []ServerLine{}
	}
	if start > 0 {
		next := items[start].ID
		page.NextCursor = &next
	}
	return page, nil
}

func (w *workspace) hasServerLine(id, network string) bool {
	if w.store != nil {
		found, err := w.store.HasServerLine(w.ownerID, network, id)
		if err != nil {
			w.storageErr = err
			return true
		}
		return found
	}
	for _, line := range w.serverLogs[network] {
		if line.ID == id {
			return true
		}
	}
	return false
}
func (m *Manager) SubscribeTrace(owner, network, after string) ([]ServerLine, <-chan ServerLine, bool, func(), error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.networks[network]; !ok {
		return nil, nil, false, nil, ErrNotFound
	}
	if len(w.traceSubscribers[network]) >= 8 {
		return nil, nil, false, nil, ErrBusy
	}
	if w.traceSubscribers[network] == nil {
		w.traceSubscribers[network] = map[chan ServerLine]struct{}{}
	}
	items := w.trace[network]
	start := 0
	valid := after == ""
	if after != "" {
		for i, v := range items {
			if v.ID == after {
				start = i + 1
				valid = true
				break
			}
		}
	}
	ch := make(chan ServerLine, 64)
	w.traceSubscribers[network][ch] = struct{}{}
	var once sync.Once
	return clone(items[start:]), ch, valid, func() {
		once.Do(func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			if _, ok := w.traceSubscribers[network][ch]; ok {
				delete(w.traceSubscribers[network], ch)
				close(ch)
			}
		})
	}, nil
}

func safeISupportParams(params []string) []string {
	if len(params) < 2 {
		return nil
	}
	return params[1 : len(params)-1]
}

// Called with the workspace mutex held.
func (w *workspace) closeTraceSubscribers() {
	for network, subscribers := range w.traceSubscribers {
		for ch := range subscribers {
			close(ch)
			delete(subscribers, ch)
		}
		delete(w.traceSubscribers, network)
	}
}
