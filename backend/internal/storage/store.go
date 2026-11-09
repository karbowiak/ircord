package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/gofrs/flock"
	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("record not found")

type Store struct {
	DB     *sql.DB
	cipher cipher.AEAD
	lock   *flock.Flock
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "backend.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		return nil, fmt.Errorf("data directory already has a running backend")
	}
	fail := func(err error) (*Store, error) { _ = lock.Unlock(); return nil, err }
	keyPath := filepath.Join(dir, "master.key")
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return fail(err)
		}
		var f *os.File
		f, err = os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fail(err)
		}
		_, err = f.Write(key)
		if err == nil {
			err = f.Sync()
		}
		_ = f.Close()
	}
	if err != nil {
		return fail(err)
	}
	if len(key) != 32 {
		return fail(fmt.Errorf("invalid master.key"))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fail(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fail(err)
	}
	path := filepath.Join(dir, "ircord.db")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fail(err)
	}
	file.Close()
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=foreign_keys(ON)&_pragma=synchronous(FULL)")
	if err != nil {
		return fail(err)
	}
	// SQLite serializes writes. A short transaction owns the account sequence and
	// its entity mutation; readers use the same handle without retrying partial state.
	db.SetMaxOpenConns(1)
	s := &Store{DB: db, cipher: aead, lock: lock}
	if err = s.migrate(dir); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { err := s.DB.Close(); _ = s.lock.Unlock(); return err }
func (s *Store) seal(owner, network string, raw []byte) []byte {
	nonce := make([]byte, s.cipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return s.cipher.Seal(nonce, nonce, raw, []byte(owner+"/"+network))
}
func (s *Store) open(owner, network string, raw []byte) ([]byte, error) {
	n := s.cipher.NonceSize()
	if len(raw) < n {
		return nil, errors.New("invalid encrypted configuration")
	}
	return s.cipher.Open(nil, raw[:n], raw[n:], []byte(owner+"/"+network))
}

func (s *Store) Commit(owner, kind string, data any) (model.Event, error) {
	tx, err := s.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return model.Event{}, err
	}
	defer tx.Rollback()
	raw, err := json.Marshal(data)
	if err != nil {
		return model.Event{}, err
	}
	var seq uint64
	if err = tx.QueryRow("UPDATE accounts SET seq=seq+1 WHERE id=? AND deleted=0 RETURNING seq", owner).Scan(&seq); err != nil {
		return model.Event{}, err
	}
	entityKind, entityID := "", ""
	switch v := data.(type) {
	case model.Invitation:
		_, err = tx.Exec("INSERT INTO invitations(owner,network,channel_key,id,channel,inviter_id,inviter_nick,created_ns,rejected) VALUES(?,?,?,?,?,?,?,?,0) ON CONFLICT(owner,network,channel_key) DO NOTHING", owner, v.NetworkID, v.ChannelKey, v.ID, v.Channel, v.InviterID, v.InviterNick, v.CreatedAt.UnixNano())
		if err != nil {
			return model.Event{}, err
		}
	case model.InvitationRemoval:
		if v.Rejected {
			_, err = tx.Exec("UPDATE invitations SET rejected=1 WHERE owner=? AND id=?", owner, v.ID)
		} else {
			_, err = tx.Exec("DELETE FROM invitations WHERE owner=? AND id=? AND rejected=0", owner, v.ID)
		}
		if err != nil {
			return model.Event{}, err
		}
	case model.Removal:
		if kind == "network.removed" {
			if _, err = tx.Exec("DELETE FROM invitations WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			for _, table := range []string{"messages", "outbox", "history_state", "conversation_read", "reactions", "message_mentions", "message_links"} {
				if _, err = tx.Exec("DELETE FROM "+table+" WHERE owner=? AND conversation IN (SELECT id FROM entities WHERE owner=? AND kind='conversation' AND json_extract(payload,'$.networkId')=?)", owner, owner, v.ID); err != nil {
					return model.Event{}, err
				}
			}
			// Bookmarks name their network directly, so ones into conversations closed
			// earlier (whose entity is already gone) are removed too.
			if _, err = tx.Exec("DELETE FROM saved_messages WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM inbox_items WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM entities WHERE owner=? AND ((kind='conversation' AND json_extract(payload,'$.networkId')=?) OR (kind IN ('network','connection') AND id=?) OR (kind='member' AND substr(id,1,length(?)+1)=?||':'))", owner, v.ID, v.ID, v.ID, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM channel_cache_meta WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM networks WHERE owner=? AND id=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM app_data WHERE owner=? AND kind=?", owner, profilePublishedKind(v.ID)); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM server_lines WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			// Previous replay frames can contain this network's history. A stale device
			// must resnapshot after deletion instead of replaying the removed content.
			if _, err = tx.Exec("DELETE FROM events WHERE owner=?", owner); err != nil {
				return model.Event{}, err
			}
		} else if kind == "conversation.removed" {
			if _, err = tx.Exec("DELETE FROM entities WHERE owner=? AND kind='conversation' AND id=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
		}

	case model.SavedMessage:
		_, err = tx.Exec("INSERT INTO saved_messages(owner,message,conversation,network,saved_ns) VALUES(?,?,?,?,?) ON CONFLICT(owner,message) DO NOTHING", owner, v.MessageID, v.ConversationID, v.NetworkID, v.SavedAt.UnixNano())
		if err != nil {
			return model.Event{}, err
		}
	case model.InboxEntry:
		_, err = tx.Exec("INSERT INTO inbox_items(owner,message,conversation,network,sender,reasons,created_ns) VALUES(?,?,?,?,?,?,?) ON CONFLICT(owner,message) DO UPDATE SET reasons=reasons|excluded.reasons", owner, v.MessageID, v.ConversationID, v.NetworkID, v.SenderID, inboxBits(v.Reasons), v.CreatedAt.UnixNano())
		if err == nil {
			err = pruneInbox(tx, owner, time.Now())
		}
		if err != nil {
			return model.Event{}, err
		}
	case model.InboxChange:
		dismissed := sql.NullInt64{}
		if v.Dismissed {
			dismissed = sql.NullInt64{Int64: time.Now().UnixNano(), Valid: true}
		}
		if _, err = tx.Exec("UPDATE inbox_items SET dismissed_ns=? WHERE owner=? AND message=?", dismissed, owner, v.MessageID); err != nil {
			return model.Event{}, err
		}
	case model.SavedRemoval:
		if _, err = tx.Exec("DELETE FROM saved_messages WHERE owner=? AND message=?", owner, v.MessageID); err != nil {
			return model.Event{}, err
		}
	case model.Member:
		entityKind = "member"
		entityID = v.ID
		if v.Ignored {
			_, err = tx.Exec("INSERT INTO ignored_senders(owner,member) VALUES(?,?) ON CONFLICT DO NOTHING", owner, v.ID)
		} else {
			_, err = tx.Exec("DELETE FROM ignored_senders WHERE owner=? AND member=?", owner, v.ID)
		}
		if err != nil {
			return model.Event{}, err
		}
	case model.ConnectionInfo:
		entityKind = "connection"
		entityID = v.NetworkID
	case model.ServerLogBatch:
		var base int64
		if err = tx.QueryRow("SELECT coalesce(max(seq),0) FROM server_lines WHERE owner=? AND network=?", owner, v.NetworkID).Scan(&base); err != nil {
			return model.Event{}, err
		}
		for i, line := range v.Lines {
			payload, _ := json.Marshal(line)
			if _, err = tx.Exec("INSERT INTO server_lines(owner,network,id,seq,payload) VALUES(?,?,?,?,?) ON CONFLICT(owner,network,id) DO NOTHING", owner, v.NetworkID, line.ID, base+int64(i)+1, payload); err != nil {
				return model.Event{}, err
			}
		}
		if _, err = tx.Exec("DELETE FROM server_lines WHERE owner=? AND network=? AND seq<coalesce((SELECT seq FROM server_lines WHERE owner=? AND network=? ORDER BY seq DESC LIMIT 1 OFFSET 499),0)", owner, v.NetworkID, owner, v.NetworkID); err != nil {
			return model.Event{}, err
		}
	case model.ServerLine:
		var logSeq int64
		if err = tx.QueryRow("SELECT coalesce(max(seq),0)+1 FROM server_lines WHERE owner=? AND network=?", owner, v.NetworkID).Scan(&logSeq); err != nil {
			return model.Event{}, err
		}
		if _, err = tx.Exec("INSERT INTO server_lines(owner,network,id,seq,payload) VALUES(?,?,?,?,?)", owner, v.NetworkID, v.ID, logSeq, raw); err != nil {
			return model.Event{}, err
		}
		if _, err = tx.Exec("DELETE FROM server_lines WHERE owner=? AND network=? AND seq<coalesce((SELECT seq FROM server_lines WHERE owner=? AND network=? ORDER BY seq DESC LIMIT 1 OFFSET 499),0)", owner, v.NetworkID, owner, v.NetworkID); err != nil {
			return model.Event{}, err
		}
	case model.Network:
		entityKind = "network"
		entityID = v.ID
	case model.Conversation:
		entityKind = "conversation"
		entityID = v.ID
	case model.Preferences:
		entityKind = "preferences"
		entityID = "self"
	case model.Message:
		if err = positionMessage(tx, owner, &v); err != nil {
			return model.Event{}, err
		}
		raw, _ = json.Marshal(v)
		if !v.Historical && v.UpstreamID != "" && (v.Kind == "message" || v.Kind == "action" || v.Kind == "notice") {
			if _, err = tx.Exec("INSERT INTO history_state(owner,conversation,synced_id,synced_ns,gap_open) VALUES(?,?,?,?,0) ON CONFLICT(owner,conversation) DO UPDATE SET synced_id=excluded.synced_id,synced_ns=excluded.synced_ns WHERE gap_open=0 AND excluded.synced_ns>=synced_ns", owner, v.ConversationID, v.UpstreamID, v.CreatedAt.UnixNano()); err != nil {
				return model.Event{}, err
			}
		}
		if err = putMessage(tx, owner, v); err != nil {
			return model.Event{}, err
		}
		if v.ClientID != "" {
			state := "sent"
			if v.UpstreamID == "" {
				state = "unconfirmed"
			}
			if _, err = tx.Exec("UPDATE outbox SET state=?,message_id=? WHERE owner=? AND client_id=?", state, v.ID, owner, v.ClientID); err != nil {
				return model.Event{}, err
			}
		}
	}
	if entityKind != "" {
		if _, err = tx.Exec("INSERT INTO entities(owner,kind,id,payload) VALUES(?,?,?,?) ON CONFLICT(owner,kind,id) DO UPDATE SET payload=excluded.payload", owner, entityKind, entityID, raw); err != nil {
			return model.Event{}, err
		}
	}
	if kind == "notification-settings.updated" {
		if err = syncIgnored(tx, owner, raw); err != nil {
			return model.Event{}, err
		}
	}
	if kind == "appearance.updated" || kind == "library.updated" || (kind == "client-preferences.updated" || kind == "notification-settings.updated") {
		if _, err = tx.Exec("INSERT INTO app_data(owner,kind,payload,updated_ns) VALUES(?,?,?,?) ON CONFLICT(owner,kind) DO UPDATE SET payload=excluded.payload,updated_ns=excluded.updated_ns", owner, kind, raw, time.Now().UnixNano()); err != nil {
			return model.Event{}, err
		}
	}
	eventData := raw
	if batch, ok := data.(model.ServerLogBatch); ok {
		eventData, _ = json.Marshal(map[string]string{"networkId": batch.NetworkID})
	}
	if kind == "appearance.updated" || kind == "library.updated" || kind == "client-preferences.updated" || kind == "notification-settings.updated" {
		sum := sha256.Sum256(raw)
		eventData, _ = json.Marshal(map[string]string{"revision": hex.EncodeToString(sum[:])})
	}
	event := model.Event{V: 1, Seq: seq, Type: kind, Data: eventData}
	eventRaw, _ := json.Marshal(event)
	if _, err = tx.Exec("INSERT INTO events(owner,seq,created_ns,payload) VALUES(?,?,?,?)", owner, seq, time.Now().UnixNano(), eventRaw); err != nil {
		return model.Event{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.Event{}, err
	}
	return event, nil
}
func putMessage(q interface {
	Exec(string, ...any) (sql.Result, error)
	QueryRow(string, ...any) *sql.Row
}, owner string, msg model.Message) error {
	if err := positionMessage(q, owner, &msg); err != nil {
		return err
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = q.Exec("INSERT INTO messages(owner,conversation,id,upstream,created_ns,text,sender,kind,payload,position) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(owner,id) DO UPDATE SET payload=excluded.payload,text=excluded.text,upstream=excluded.upstream,created_ns=excluded.created_ns,sender=excluded.sender,kind=excluded.kind", owner, msg.ConversationID, msg.ID, msg.UpstreamID, msg.CreatedAt.UnixNano(), msg.Text, msg.SenderID, msg.Kind, raw, msg.Position)
	if err == nil && msg.Mention {
		_, err = q.Exec("INSERT INTO message_mentions(owner,conversation,message) VALUES(?,?,?) ON CONFLICT DO NOTHING", owner, msg.ConversationID, msg.ID)
	}
	if err == nil {
		err = indexMessage(q, owner, msg)
	}
	return err
}

// SaveHistory stores a message and its position in a single transaction.
func (s *Store) SaveHistory(owner string, msg model.Message) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = putMessage(tx, owner, msg); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Message(owner, conversation, id string) (model.Message, error) {
	return s.MessageContext(context.Background(), owner, conversation, id)
}

func (s *Store) MessageContext(ctx context.Context, owner, conversation, id string) (model.Message, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, "SELECT payload FROM messages WHERE owner=? AND conversation=? AND (id=? OR upstream=?)", owner, conversation, id, id).Scan(&raw)
	var msg model.Message
	if errors.Is(err, sql.ErrNoRows) {
		return msg, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &msg)
	}
	return msg, err
}

type cursor struct {
	Position int64  `json:"p,omitempty"`
	At       int64  `json:"at"`
	ID       string `json:"id"`
}

func Cursor(msg model.Message) string {
	raw, _ := json.Marshal(cursor{At: msg.CreatedAt.UnixNano(), ID: msg.ID, Position: msg.Position})
	return base64.RawURLEncoding.EncodeToString(raw)
}
func ParseCursor(raw string) (int64, string, error) {
	if len(raw) > 1024 {
		return 0, "", errors.New("invalid cursor")
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	var c cursor
	if err != nil || json.Unmarshal(b, &c) != nil || c.At <= 0 || c.ID == "" {
		return 0, "", errors.New("invalid cursor")
	}
	return c.At, c.ID, nil
}
func (s *Store) History(owner, conversation, before string, limit int) (model.MessagePage, error) {
	return s.HistoryContext(context.Background(), owner, conversation, before, limit)
}

func (s *Store) HistoryContext(ctx context.Context, owner, conversation, before string, limit int) (model.MessagePage, error) {
	page := model.MessagePage{Items: []model.Message{}}
	query := "SELECT payload FROM messages WHERE owner=? AND conversation=?"
	args := []any{owner, conversation}
	if before != "" {
		at, id, err := ParseCursor(before)
		var c cursor
		decoded, _ := base64.RawURLEncoding.DecodeString(before)
		_ = json.Unmarshal(decoded, &c)
		if err != nil {
			return page, err
		}
		query += " AND (created_ns<? OR (created_ns=? AND (position<? OR (position=? AND id<?))))"
		args = append(args, at, at, c.Position, c.Position, id)
	}
	query += " ORDER BY created_ns DESC,position DESC,id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var msg model.Message
		if err = rows.Scan(&raw); err != nil {
			return page, err
		}
		if err = json.Unmarshal(raw, &msg); err != nil {
			return page, err
		}
		page.Items = append(page.Items, msg)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		next := Cursor(page.Items[len(page.Items)-1])
		page.NextCursor = &next
	}
	slices.Reverse(page.Items)
	return page, nil
}

// HistoryAround reads a window of at most limit messages centred on message:
// up to half of it older, then the message, then newer ones. It reports
// whether older and newer messages exist beyond the window, and ErrNotFound
// when message is not a stored message of the conversation.
func (s *Store) HistoryAround(owner, conversation, message string, limit int) (page model.MessagePage, older bool, err error) {
	return s.HistoryAroundContext(context.Background(), owner, conversation, message, limit)
}

func (s *Store) HistoryAroundContext(ctx context.Context, owner, conversation, message string, limit int) (page model.MessagePage, older bool, err error) {
	page = model.MessagePage{Items: []model.Message{}}
	var at, position int64
	var id string
	err = s.DB.QueryRowContext(ctx, "SELECT created_ns,position,id FROM messages WHERE owner=? AND conversation=? AND id=?", owner, conversation, message).Scan(&at, &position, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return page, false, ErrNotFound
	}
	if err != nil {
		return page, false, err
	}
	read := func(query string, n int) ([]model.Message, error) {
		rows, err := s.DB.QueryContext(ctx, query, owner, conversation, at, at, position, position, id, n)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []model.Message{}
		for rows.Next() {
			var raw []byte
			var msg model.Message
			if err = rows.Scan(&raw); err != nil {
				return nil, err
			}
			if err = json.Unmarshal(raw, &msg); err != nil {
				return nil, err
			}
			out = append(out, msg)
		}
		return out, rows.Err()
	}
	before := (limit - 1) / 2
	after := limit - 1 - before
	olderItems, err := read("SELECT payload FROM messages WHERE owner=? AND conversation=? AND (created_ns<? OR (created_ns=? AND (position<? OR (position=? AND id<?)))) ORDER BY created_ns DESC,position DESC,id DESC LIMIT ?", before+1)
	if err != nil {
		return page, false, err
	}
	newerItems, err := read("SELECT payload FROM messages WHERE owner=? AND conversation=? AND (created_ns>? OR (created_ns=? AND (position>? OR (position=? AND id>?)))) ORDER BY created_ns,position,id LIMIT ?", after+1)
	if err != nil {
		return page, false, err
	}
	older = len(olderItems) > before
	if older {
		olderItems = olderItems[:before]
	}
	page.HasNewer = len(newerItems) > after
	if page.HasNewer {
		newerItems = newerItems[:after]
	}
	target, err := s.MessageContext(ctx, owner, conversation, id)
	if err != nil {
		return page, false, err
	}
	slices.Reverse(olderItems)
	page.Items = append(append(olderItems, target), newerItems...)
	if older {
		next := Cursor(page.Items[0])
		page.NextCursor = &next
	}
	return page, older, nil
}
func (s *Store) Replay(owner string, after uint64, limit int) ([]model.Event, bool, error) {
	var head uint64
	if err := s.DB.QueryRow("SELECT seq FROM accounts WHERE id=?", owner).Scan(&head); err != nil {
		return nil, false, err
	}
	var oldest sql.NullInt64
	if err := s.DB.QueryRow("SELECT MIN(seq) FROM events WHERE owner=?", owner).Scan(&oldest); err != nil {
		return nil, false, err
	}
	if after > head || (!oldest.Valid && after != head) || (oldest.Valid && after+1 < uint64(oldest.Int64)) {
		return nil, false, nil
	}
	rows, err := s.DB.Query("SELECT payload FROM events WHERE owner=? AND seq>? ORDER BY seq LIMIT ?", owner, after, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	events := []model.Event{}
	for rows.Next() {
		var raw []byte
		var e model.Event
		if err = rows.Scan(&raw); err != nil {
			return nil, false, err
		}
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, false, err
		}
		events = append(events, e)
	}
	if len(events) > limit {
		return nil, false, nil
	}
	return events, true, rows.Err()
}
func (s *Store) Head(owner string) (uint64, error) {
	var n uint64
	err := s.DB.QueryRow("SELECT seq FROM accounts WHERE id=?", owner).Scan(&n)
	return n, err
}
func (s *Store) SaveNetwork(owner string, cfg model.NetworkConfig, enabled bool) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	encrypted := s.seal(owner, cfg.ID, raw)
	_, err = s.DB.Exec("INSERT INTO networks(owner,id,config,enabled) VALUES(?,?,?,?) ON CONFLICT(owner,id) DO UPDATE SET config=excluded.config,enabled=excluded.enabled", owner, cfg.ID, encrypted, enabled)
	return err
}

type SavedNetwork struct {
	Owner   string
	Config  model.NetworkConfig
	Enabled bool
}

func (s *Store) Networks(owner string) ([]SavedNetwork, error) {
	query := "SELECT owner,id,config,enabled FROM networks"
	args := []any{}
	if owner != "" {
		query += " WHERE owner=?"
		args = append(args, owner)
	}
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SavedNetwork{}
	for rows.Next() {
		var owner, id string
		var raw []byte
		var enabled bool
		if err = rows.Scan(&owner, &id, &raw, &enabled); err != nil {
			return nil, err
		}
		raw, err = s.open(owner, id, raw)
		if err != nil {
			return nil, errors.New("cannot decrypt network configuration: check master.key")
		}
		var cfg model.NetworkConfig
		if err = json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		result = append(result, SavedNetwork{owner, cfg, enabled})
	}
	return result, rows.Err()
}
func (s *Store) Entities(owner string) (map[string][]json.RawMessage, error) {
	rows, err := s.DB.Query("SELECT kind,payload FROM entities WHERE owner=?", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]json.RawMessage{}
	for rows.Next() {
		var kind string
		var raw []byte
		if err = rows.Scan(&kind, &raw); err != nil {
			return nil, err
		}
		out[kind] = append(out[kind], json.RawMessage(raw))
	}
	return out, rows.Err()
}
func (s *Store) AppData(owner, kind string) (json.RawMessage, error) {
	var raw []byte
	err := s.DB.QueryRow("SELECT payload FROM app_data WHERE owner=? AND kind=?", owner, kind).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return raw, err
}
func (s *Store) PruneEvents(before time.Time) error {
	_, err := s.DB.Exec("DELETE FROM events WHERE created_ns<?", before.UnixNano())
	return err
}

// Position orders messages which share IRC's millisecond server-time precision.
// Older batches reserve a range before the current window, in server batch order.
func positionMessage(q interface{ QueryRow(string, ...any) *sql.Row }, owner string, msg *model.Message) error {
	var existing int64
	err := q.QueryRow("SELECT position FROM messages WHERE owner=? AND id=?", owner, msg.ID).Scan(&existing)
	if err == nil {
		msg.Position = existing
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if msg.Position != 0 {
		return nil
	}
	return q.QueryRow("SELECT coalesce(max(position),0)+1 FROM messages WHERE owner=? AND conversation=?", owner, msg.ConversationID).Scan(&msg.Position)
}
func (s *Store) HistoryBase(owner, conversation string) (int64, error) {
	var base int64
	err := s.DB.QueryRow("SELECT min(coalesce((SELECT min(position) FROM messages WHERE owner=? AND conversation=?),0),0)-1000000", owner, conversation).Scan(&base)
	return base, err
}

func syncIgnored(tx *sql.Tx, owner string, raw []byte) error {
	policy := notify.Compile(raw)
	rows, err := tx.Query("SELECT payload FROM entities WHERE owner=? AND kind='member'", owner)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			rows.Close()
			return err
		}
		var member model.Member
		if err = json.Unmarshal(data, &member); err != nil {
			rows.Close()
			return err
		}
		if member.ID != owner && policy.Ignored(member.Nick, member.Username, member.Host) {
			ids = append(ids, member.ID)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM ignored_senders WHERE owner=?", owner); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = tx.Exec("INSERT INTO ignored_senders(owner,member) VALUES(?,?)", owner, id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) SealSecret(owner, purpose string, raw []byte) []byte {
	return s.seal(owner, purpose, raw)
}
func (s *Store) OpenSecret(owner, purpose string, raw []byte) ([]byte, error) {
	return s.open(owner, purpose, raw)
}
