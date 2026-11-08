package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"golang.org/x/crypto/hkdf"
	"ircord/backend/internal/activity"
	"ircord/backend/internal/storage"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func clientSubscription(t *testing.T, endpoint string) (Subscription, *ecdh.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return Subscription{Endpoint: endpoint, Keys: webpush.Keys{Auth: base64.RawURLEncoding.EncodeToString(auth), P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())}}, key, auth
}
func decrypt(t *testing.T, body []byte, key *ecdh.PrivateKey, auth []byte) Payload {
	t.Helper()
	if len(body) < 86 {
		t.Fatal("missing Web Push encryption header")
	}
	salt := body[:16]
	length := int(body[20])
	public, err := ecdh.P256().NewPublicKey(body[21 : 21+length])
	if err != nil {
		t.Fatal(err)
	}
	shared, err := key.ECDH(public)
	if err != nil {
		t.Fatal(err)
	}
	prk := hkdf.Extract(sha256.New, shared, auth)
	info := append([]byte("WebPush: info\x00"), key.PublicKey().Bytes()...)
	info = append(info, public.Bytes()...)
	ikm := make([]byte, 32)
	io.ReadFull(hkdf.Expand(sha256.New, prk, info), ikm)
	prk = hkdf.Extract(sha256.New, ikm, salt)
	cek, nonce := make([]byte, 16), make([]byte, 12)
	io.ReadFull(hkdf.Expand(sha256.New, prk, []byte("Content-Encoding: aes128gcm\x00")), cek)
	io.ReadFull(hkdf.Expand(sha256.New, prk, []byte("Content-Encoding: nonce\x00")), nonce)
	block, _ := aes.NewCipher(cek)
	aead, _ := cipher.NewGCM(block)
	plain, err := aead.Open(nil, nonce, body[21+length:], nil)
	if err != nil {
		t.Fatal(err)
	}
	plain = bytes.TrimRight(plain, "\x00")
	if len(plain) == 0 || plain[len(plain)-1] != 2 {
		t.Fatal("invalid RFC8291 record delimiter")
	}
	var value Payload
	if err = json.Unmarshal(plain[:len(plain)-1], &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func TestEncryptedDeliveryIsolationRevocationAndExpiry(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner); err != nil {
			t.Fatal(err)
		}
		if err = store.CreateSession(owner+"-session", owner, []byte(owner), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	subscription, key, secret := clientSubscription(t, "https://fcm.googleapis.com/fcm/send/alice-device")
	other, _, _ := clientSubscription(t, "https://fcm.googleapis.com/fcm/send/bob-device")
	delivered := []Payload{}
	var mu sync.Mutex
	gone := false
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(r.URL.Path, "bob-device") {
			t.Error("Alice's notification reached Bob")
		}
		if r.Header.Get("Content-Encoding") != "aes128gcm" || r.Header.Get("Authorization") == "" || r.Header.Get("TTL") != "3600" || len(r.Header.Get("Topic")) > 32 {
			t.Error("Web Push wire contract invalid")
		}
		if bytes.Contains(body, []byte("private notification")) {
			t.Error("payload was sent in plaintext")
		}
		value := decrypt(t, body, key, secret)
		mu.Lock()
		delivered = append(delivered, value)
		status := 201
		if gone {
			status = 410
		}
		mu.Unlock()
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})}
	dir := t.TempDir()
	service, err := New(Options{Store: store, DataDir: dir, Subject: "mailto:tests@example.invalid", Client: client})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	if err = service.Add(ctx, "alice", "alice-session", "test", subscription); err != nil {
		t.Fatal(err)
	}
	if err = service.Add(ctx, "bob", "bob-session", "test", other); err != nil {
		t.Fatal(err)
	}
	payload := Payload{Title: "Alice", Body: "private notification", ConversationID: "room", Tag: strings.Repeat("conversation", 10), Badge: 3}
	if err = service.Send(ctx, "alice", payload); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(delivered) != 1 || delivered[0].Body != payload.Body || delivered[0].Badge != 3 {
		t.Fatal("encrypted payload did not round-trip")
	}
	mu.Unlock()
	var encrypted []byte
	store.DB.QueryRow("SELECT payload FROM push_subscriptions WHERE owner='alice'").Scan(&encrypted)
	if bytes.Contains(encrypted, []byte(subscription.Endpoint)) || bytes.Contains(encrypted, []byte(subscription.Keys.Auth)) {
		t.Fatal("subscription credentials were not encrypted")
	}
	if err = store.RevokeSession("alice", "alice-session"); err != nil {
		t.Fatal(err)
	}
	if err = service.Send(ctx, "alice", payload); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(delivered) != 1 {
		t.Fatal("revoked session still received push")
	}
	mu.Unlock()
	if err = service.Add(ctx, "bob", "bob-session", "same browser", subscription); err != nil {
		t.Fatal(err)
	}
	if err = service.Send(ctx, "alice", payload); err != nil {
		t.Fatal(err)
	}
	// Rebind the browser endpoint, then remove it after a push-service 410.
	if err = service.Remove("bob", other.Endpoint); err != nil {
		t.Fatal(err)
	}
	gone = true
	if err = service.Send(ctx, "bob", payload); err != nil {
		t.Fatal(err)
	}
	var count int
	store.DB.QueryRow("SELECT count(*) FROM push_subscriptions WHERE owner='bob'").Scan(&count)
	if count != 0 {
		t.Fatal("expired subscription was not removed")
	}
	public := service.PublicKey()
	service.Close()
	reopened, err := New(Options{Store: store, DataDir: dir, Subject: "mailto:tests@example.invalid", Client: client})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.PublicKey() != public {
		t.Fatal("VAPID keys changed on restart")
	}
	for _, endpoint := range []string{"https://127.0.0.1/private", "https://fcm.googleapis.com.evil.invalid/send", "http://fcm.googleapis.com/send", "https://user:secret@fcm.googleapis.com/send", "https://fcm.googleapis.com:8443/send"} {
		if ValidEndpoint(endpoint) {
			t.Fatal("untrusted push destination accepted")
		}
	}
}

// Routed alerts reach only the selected sessions, and a failed targeted delivery falls back to
// the account's other devices (#84).
func TestRoutedDeliveryTargetsSessionsAndFallsBack(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "alice"); err != nil {
		t.Fatal(err)
	}
	for _, session := range []string{"laptop", "phone"} {
		if err = store.CreateSession(session, "alice", []byte(session), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	hits := []string{}
	failing := map[string]bool{}
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		device := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		mu.Lock()
		hits = append(hits, device)
		status := 201
		if failing[device] {
			status = 500
		}
		mu.Unlock()
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})}
	service, err := New(Options{Store: store, DataDir: t.TempDir(), Subject: "mailto:tests@example.invalid", Client: client})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	for _, session := range []string{"laptop", "phone"} {
		sub, _, _ := clientSubscription(t, "https://fcm.googleapis.com/fcm/send/"+session)
		if err = service.Add(ctx, "alice", session, session, sub); err != nil {
			t.Fatal(err)
		}
	}
	if !service.HasSubscription("alice", "laptop") || service.HasSubscription("alice", "tablet") || service.HasSubscription("bob", "laptop") {
		t.Fatal("HasSubscription must reflect the account's own sessions")
	}
	// take waits for n deliveries, then a moment longer so an unexpected extra one shows.
	take := func(n int) []string {
		deadline := time.Now().Add(5 * time.Second)
		for {
			mu.Lock()
			count := len(hits)
			mu.Unlock()
			if count >= n || time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		out := hits
		hits = nil
		return out
	}
	alert := activity.Alert{ConversationID: "dm", MessageID: "m1", Title: "bob", Body: "hi", Kind: "mention", Unread: 1, Conversations: 1}
	fellBack := make(chan struct{}, 4)
	notice := func() { fellBack <- struct{}{} }
	service.Route("alice", alert, activity.Target{Only: []string{"laptop"}, Fallback: true}, notice)
	if got := take(1); len(got) != 1 || got[0] != "laptop" || len(fellBack) != 0 {
		t.Fatalf("targeted alert should reach the laptop only: %v", got)
	}
	service.Route("alice", alert, activity.Target{Except: []string{"laptop"}}, nil)
	if got := take(1); len(got) != 1 || got[0] != "phone" {
		t.Fatalf("excluded device must not be alerted: %v", got)
	}
	mu.Lock()
	failing["laptop"] = true
	mu.Unlock()
	service.Route("alice", alert, activity.Target{Only: []string{"laptop"}, Fallback: true}, notice)
	if got := take(2); len(got) != 2 || got[0] != "laptop" || got[1] != "phone" {
		t.Fatalf("a failed targeted delivery falls back to the other devices: %v", got)
	}
	select {
	case <-fellBack:
	case <-time.After(5 * time.Second):
		t.Fatal("the router was not told about the fallback")
	}
	// Removing a subscription is reflected without a database round trip on the routing path.
	sub, _, _ := clientSubscription(t, "https://fcm.googleapis.com/fcm/send/phone")
	if err = service.Remove("alice", sub.Endpoint); err != nil {
		t.Fatal(err)
	}
	if service.HasSubscription("alice", "phone") || !service.HasSubscription("alice", "laptop") {
		t.Fatal("HasSubscription must follow Remove")
	}
	reminder := AlertPayload(activity.Alert{ConversationID: "dm", Title: "bob", Body: "latest", Unread: 10, Conversations: 3, Reminder: true})
	if reminder.Title != "10 unread notifications in 3 conversations" || reminder.Body != "bob: latest" || reminder.Tag != "dm" {
		t.Fatalf("unexpected reminder payload %+v", reminder)
	}
}

// A settings test push reaches only the requesting device; a device without a registration
// gets zero deliveries instead of a silent success (#189).
func TestSendToReachesOnlyThatSession(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "alice"); err != nil {
		t.Fatal(err)
	}
	for _, session := range []string{"laptop", "phone", "tablet"} {
		if err = store.CreateSession(session, "alice", []byte(session), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	hits := []string{}
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		hits = append(hits, r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
		mu.Unlock()
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})}
	service, err := New(Options{Store: store, DataDir: t.TempDir(), Subject: "mailto:tests@example.invalid", Client: client})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	for _, session := range []string{"laptop", "phone"} {
		sub, _, _ := clientSubscription(t, "https://fcm.googleapis.com/fcm/send/"+session)
		if err = service.Add(ctx, "alice", session, session, sub); err != nil {
			t.Fatal(err)
		}
	}
	payload := Payload{Title: "IRCord", Body: "test", Tag: "test"}
	delivered, err := service.SendTo(ctx, "alice", "phone", payload)
	if err != nil || delivered != 1 {
		t.Fatalf("SendTo(phone) = %d, %v", delivered, err)
	}
	mu.Lock()
	if len(hits) != 1 || hits[0] != "phone" {
		t.Fatalf("test push reached %v, want only the phone", hits)
	}
	hits = nil
	mu.Unlock()
	if delivered, err = service.SendTo(ctx, "alice", "tablet", payload); err != nil || delivered != 0 {
		t.Fatalf("SendTo(unregistered) = %d, %v", delivered, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(hits) != 0 {
		t.Fatalf("an unregistered device's test reached %v", hits)
	}
}
