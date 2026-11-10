// Package push owns session-bound browser subscriptions and encrypted Web Push.
package push

import (
	"context"
	"crypto/ecdh"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"ircord/backend/internal/activity"
	"ircord/backend/internal/storage"
)

type Subscription = webpush.Subscription
type Payload struct {
	MessageID      string `json:"messageId,omitempty"`
	AlertID        string `json:"alertId,omitempty"`
	Generation     string `json:"generation,omitempty"`
	Delivery       string `json:"delivery,omitempty"`
	Sound          bool   `json:"sound"`
	Preview        bool   `json:"preview"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	ConversationID string `json:"conversationId,omitempty"`
	Tag            string `json:"tag"`
	Badge          int    `json:"badge"`
}
type keys struct {
	Private string `json:"privateKey"`
	Public  string `json:"publicKey"`
}
type Options struct {
	Store            *storage.Store
	DataDir, Subject string
	Client           *http.Client
	ValidateEndpoint func(string) bool
}
type job struct {
	owner   string
	payload Payload
	target  activity.Target
	// fellBack is told when a targeted delivery failed and the other devices were tried.
	fellBack func()
}
type Service struct {
	options Options
	keys    keys
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	jobs    map[string]job
	// registered mirrors which sessions have a subscription (owner → session), so routing can
	// ask without a database round trip. It holds no endpoint or key material.
	registeredMu sync.RWMutex
	registered   map[string]map[string]bool
	queue        chan string
	slots        chan struct{}
}

var ErrBusy = errors.New("notification queue is full; try again shortly")

func New(options Options) (*Service, error) {
	subject, err := url.Parse(options.Subject)
	if err != nil || subject.Scheme != "mailto" && subject.Scheme != "https" || subject.Scheme == "https" && subject.Host == "" || subject.Scheme == "mailto" && subject.Opaque == "" {
		return nil, fmt.Errorf("IRCORD_PUSH_SUBJECT must be a mailto: or HTTPS contact URL")
	}
	if options.Store == nil {
		return nil, fmt.Errorf("push requires persistent storage")
	}
	if options.ValidateEndpoint == nil {
		options.ValidateEndpoint = ValidEndpoint
	}
	if options.Client == nil {
		options.Client = safeHTTPClient()
	}
	client := *options.Client
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	options.Client = &client
	if err = os.MkdirAll(options.DataDir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(options.DataDir, "vapid.json")
	raw, err := os.ReadFile(path)
	var k keys
	if os.IsNotExist(err) {
		k.Private, k.Public, err = webpush.GenerateVAPIDKeys()
		if err != nil {
			return nil, err
		}
		raw, _ = json.Marshal(k)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, err
		}
		if _, err = file.Write(raw); err == nil {
			err = file.Sync()
		}
		file.Close()
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &k); err != nil {
		return nil, fmt.Errorf("invalid VAPID key file")
	}
	private, err := base64.RawURLEncoding.DecodeString(k.Private)
	if err != nil {
		return nil, fmt.Errorf("invalid VAPID private key")
	}
	key, err := ecdh.P256().NewPrivateKey(private)
	if err != nil || base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()) != k.Public {
		return nil, fmt.Errorf("VAPID key pair does not match")
	}
	_, err = options.Store.DB.Exec(`CREATE TABLE IF NOT EXISTS push_subscriptions(id TEXT PRIMARY KEY,owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,session TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,device TEXT NOT NULL,payload BLOB NOT NULL,created_ns INTEGER NOT NULL); CREATE INDEX IF NOT EXISTS push_by_owner ON push_subscriptions(owner);`)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{options: options, keys: k, ctx: ctx, cancel: cancel, jobs: map[string]job{}, queue: make(chan string, 1024), slots: make(chan struct{}, 4)}
	if err = s.reload(); err != nil {
		cancel()
		return nil, err
	}
	for i := 0; i < 4; i++ {
		s.wg.Add(1)
		go s.worker()
	}
	return s, nil
}
func ValidEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 4096 || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.Port() != "" && u.Port() != "443" || u.Fragment != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, domain := range []string{"fcm.googleapis.com", "android.googleapis.com", "mozilla.com", "mozaws.net", "push.apple.com", "notify.windows.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
func (s *Service) PublicKey() string { return s.keys.Public }
func decode(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
}
func (s *Service) Add(ctx context.Context, owner, session, device string, sub Subscription) error {
	if !s.options.ValidateEndpoint(sub.Endpoint) || len(device) > 120 || strings.ContainsAny(device, "\r\n\x00") {
		return errors.New("invalid push subscription")
	}
	public, err := decode(sub.Keys.P256dh)
	if err != nil {
		return errors.New("invalid push encryption key")
	}
	if _, err = ecdh.P256().NewPublicKey(public); err != nil {
		return errors.New("invalid push encryption key")
	}
	secret, err := decode(sub.Keys.Auth)
	if err != nil || len(secret) != 16 {
		return errors.New("invalid push authentication key")
	}
	if !s.options.Store.SessionActive(owner, session) {
		return errors.New("session is no longer active")
	}
	sum := sha256.Sum256([]byte(sub.Endpoint))
	id := hex.EncodeToString(sum[:])
	raw, _ := json.Marshal(sub)
	encrypted := s.options.Store.SealSecret(owner, "push/"+id, raw)
	// One browser endpoint belongs to one current account/session. Rebinding it
	// prevents a previous signed-out account from notifying the next browser user.
	tx, err := s.options.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM push_subscriptions WHERE owner=? AND session IN (SELECT id FROM sessions WHERE revoked=1 OR expires_ns<=?)", owner, time.Now().UnixNano()); err != nil {
		return err
	}
	var n int
	if err = tx.QueryRow("SELECT count(*) FROM push_subscriptions WHERE owner=? AND id!=?", owner, id).Scan(&n); err != nil {
		return err
	}
	if n >= 20 {
		return errors.New("this account already has 20 push devices")
	}
	_, err = tx.Exec("INSERT INTO push_subscriptions(id,owner,session,device,payload,created_ns) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET owner=excluded.owner,session=excluded.session,device=excluded.device,payload=excluded.payload", id, owner, session, device, encrypted, time.Now().UnixNano())
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.reload()
}
func (s *Service) Remove(owner, endpoint string) error {
	sum := sha256.Sum256([]byte(endpoint))
	_, err := s.options.Store.DB.Exec("DELETE FROM push_subscriptions WHERE owner=? AND id=?", owner, hex.EncodeToString(sum[:]))
	if err != nil {
		return err
	}
	return s.reload()
}

// reload rebuilds the in-memory view of which sessions have a subscription. Subscriptions
// change rarely (a device turning push on or off), so the whole small table is read.
func (s *Service) reload() error {
	rows, err := s.options.Store.DB.Query("SELECT owner,session FROM push_subscriptions")
	if err != nil {
		return err
	}
	defer rows.Close()
	registered := map[string]map[string]bool{}
	for rows.Next() {
		var owner, session string
		if err = rows.Scan(&owner, &session); err != nil {
			return err
		}
		if registered[owner] == nil {
			registered[owner] = map[string]bool{}
		}
		registered[owner][session] = true
	}
	if err = rows.Err(); err != nil {
		return err
	}
	s.registeredMu.Lock()
	s.registered = registered
	s.registeredMu.Unlock()
	return nil
}

// Notify queues a notification for every device of the account.
func (s *Service) Notify(owner string, payload Payload) error {
	return s.notify(owner, payload, activity.Target{}, nil)
}

// notify queues a notification for the target's devices. A queued job with the same account,
// tag and target is replaced, so a burst coalesces to its latest item.
func (s *Service) notify(owner string, payload Payload, target activity.Target, fellBack func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return s.ctx.Err()
	}
	key := owner + "/" + payload.Tag + "/" + strings.Join(target.Only, ",") + "/" + strings.Join(target.Except, ",")
	if target.Only != nil {
		key += "/only"
	}
	if payload.AlertID != "" {
		key += "/" + payload.AlertID
	}
	if _, ok := s.jobs[key]; ok {
		s.jobs[key] = job{owner, payload, target, fellBack}
		return nil
	}
	if len(s.jobs) >= 1024 {
		return ErrBusy
	}
	s.jobs[key] = job{owner, payload, target, fellBack}
	s.queue <- key
	return nil
}

// AlertPayload is the Web Push form of a routed alert. A reminder leads with how many
// notifications are unread and carries the latest one.
func AlertPayload(alert activity.Alert) Payload {
	payload := Payload{Title: alert.Title, Body: alert.Body, ConversationID: alert.ConversationID, Tag: alert.ConversationID, Badge: alert.Badge, MessageID: alert.MessageID, AlertID: alert.AlertID, Generation: alert.Generation, Delivery: alert.Delivery, Sound: alert.Sound, Preview: alert.Preview}
	if alert.Reminder {
		payload.Title = fmt.Sprintf("%d unread notifications", alert.Unread)
		if alert.Conversations > 1 {
			payload.Title += fmt.Sprintf(" in %d conversations", alert.Conversations)
		}
		payload.Body = alert.Title + ": " + alert.Body
	}
	return payload
}

// Route queues a routed alert for the target's devices; it satisfies activity.Pusher.
func (s *Service) Route(owner string, alert activity.Alert, target activity.Target, fellBack func()) {
	if alert.Delivery == "sound" {
		return
	}
	_ = s.notify(owner, AlertPayload(alert), target, fellBack)
}

// TryRoute lets the durable scheduler retain its cursor when the bounded push
// queue is full, rather than recording a notification that was never queued.
func (s *Service) TryRoute(owner string, alert activity.Alert, target activity.Target) bool {
	if alert.Delivery == "sound" {
		return false
	}
	return s.notify(owner, AlertPayload(alert), target, nil) == nil
}

// DeliveryReceipts tells the router that enqueuing is not delivery. A routed
// item's durable receipt is recorded after the provider accepts it.
func (s *Service) DeliveryReceipts() bool { return true }

// HasSubscription reports whether the session has a push subscription registered.
func (s *Service) HasSubscription(owner, session string) bool {
	s.registeredMu.RLock()
	defer s.registeredMu.RUnlock()
	return s.registered[owner][session]
}
func (s *Service) worker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case key := <-s.queue:
			if s.ctx.Err() != nil {
				return
			}
			s.mu.Lock()
			j, ok := s.jobs[key]
			delete(s.jobs, key)
			s.mu.Unlock()
			if ok {
				ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
				delivered, _ := s.send(ctx, j.owner, j.payload, j.target)
				if j.target.Done != nil && len(j.target.Only) == 1 {
					j.target.Done(j.target.Only[0], delivered > 0)
				}
				if delivered == 0 && j.target.Only != nil && j.target.Fallback {
					// The targeted device could not be reached: fall back to the others.
					_, _ = s.send(ctx, j.owner, j.payload, activity.Target{Except: j.target.Only})
					if j.fellBack != nil {
						j.fellBack()
					}
				}
				cancel()
			}
		}
	}
}

// Send delivers to every device of the account now.
func (s *Service) Send(ctx context.Context, owner string, payload Payload) error {
	_, err := s.send(ctx, owner, payload, activity.Target{})
	return err
}

// SendTo delivers to one session's devices now and reports how many accepted it. Zero with no
// error means nothing accepted it: the session has no registration, or the push service just
// reported it gone (and it was removed).
func (s *Service) SendTo(ctx context.Context, owner, session string, payload Payload) (int, error) {
	return s.send(ctx, owner, payload, activity.Target{Only: []string{session}})
}

// send delivers to the selected sessions and reports how many accepted the notification.
func (s *Service) send(ctx context.Context, owner string, payload Payload, selected activity.Target) (int, error) {
	type target struct {
		id, session string
		raw         []byte
	}
	rows, err := s.options.Store.DB.QueryContext(ctx, "SELECT p.id,p.session,p.payload FROM push_subscriptions p JOIN sessions s ON s.id=p.session JOIN accounts a ON a.id=p.owner WHERE p.owner=? AND s.revoked=0 AND s.expires_ns>? AND a.deleted=0 AND a.last_seen_ns>?", owner, time.Now().UnixNano(), time.Now().Add(-storage.AccountRetention).UnixNano())
	if err != nil {
		return 0, err
	}
	targets := []target{}
	for rows.Next() {
		var item target
		if err = rows.Scan(&item.id, &item.session, &item.raw); err != nil {
			rows.Close()
			return 0, err
		}
		if selected.Includes(item.session) {
			targets = append(targets, item)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	body, _ := json.Marshal(payload)
	var sendErr error
	delivered := 0
	for _, target := range targets {
		if !s.options.Store.SessionActive(owner, target.session) {
			continue
		}
		if selected.Valid != nil && !selected.Valid(target.session) {
			continue
		}
		plain, err := s.options.Store.OpenSecret(owner, "push/"+target.id, target.raw)
		if err != nil {
			return delivered, err
		}
		var sub Subscription
		if json.Unmarshal(plain, &sub) != nil || !s.options.ValidateEndpoint(sub.Endpoint) {
			continue
		}
		select {
		case s.slots <- struct{}{}:
		case <-ctx.Done():
			return delivered, ctx.Err()
		}
		sendCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		topic := sha256.Sum256([]byte(payload.Tag))
		response, err := webpush.SendNotificationWithContext(sendCtx, body, &sub, &webpush.Options{HTTPClient: s.options.Client, Subscriber: s.options.Subject, VAPIDPublicKey: s.keys.Public, VAPIDPrivateKey: s.keys.Private, TTL: 3600, Urgency: webpush.UrgencyHigh, Topic: base64.RawURLEncoding.EncodeToString(topic[:24])})
		if response != nil {
			io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			response.Body.Close()
		}
		cancel()
		<-s.slots
		if err != nil {
			sendErr = errors.New("push delivery failed")
			continue
		}
		if response.StatusCode == 404 || response.StatusCode == 410 {
			_, _ = s.options.Store.DB.Exec("DELETE FROM push_subscriptions WHERE owner=? AND id=? AND session=?", owner, target.id, target.session)
			_ = s.reload()
		} else if response.StatusCode < 200 || response.StatusCode >= 300 {
			sendErr = fmt.Errorf("push service returned HTTP %d", response.StatusCode)
		} else {
			delivered++
		}
	}
	return delivered, sendErr
}
func (s *Service) Close() { s.cancel(); s.wg.Wait(); s.options.Client.CloseIdleConnections() }
