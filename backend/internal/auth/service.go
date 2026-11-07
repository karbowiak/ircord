package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"ircord/backend/internal/storage"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

var ErrUnauthorized = errors.New("invalid account key or expired session")
var ErrUnavailable = errors.New("account storage is unavailable")
var ErrRegistration = errors.New("registration is disabled")
var ErrLimited = errors.New("too many authentication attempts; try again shortly")

type Credentials struct {
	Key string `json:"key" minLength:"1" maxLength:"128" writeOnly:"true"`
}
type Principal struct {
	Account storage.Account `json:"account"`
	Session storage.Session `json:"-"`
}
type LoginResult struct {
	Key     string          `json:"key,omitempty"`
	Account storage.Account `json:"account"`
}
type Options struct {
	TrustedProxies  []netip.Prefix
	SecureCookies   bool
	Registration    bool
	SessionLifetime time.Duration
	BeforeDelete    func(string) error
}
type Service struct {
	store    *storage.Store
	options  Options
	mu       sync.Mutex
	attempts map[string]attempt
	watchers map[string]map[string]context.CancelFunc
}
type attempt struct {
	count int
	until time.Time
}

func New(store *storage.Store, options Options) *Service {
	if options.SessionLifetime <= 0 {
		options.SessionLifetime = 90 * 24 * time.Hour
	}
	return &Service{store: store, options: options, attempts: map[string]attempt{}, watchers: map[string]map[string]context.CancelFunc{}}
}
func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

var crockford = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

func newKey() string {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	encoded := crockford.EncodeToString(raw)[:30]
	groups := []string{}
	for i := 0; i < len(encoded); i += 5 {
		groups = append(groups, encoded[i:i+5])
	}
	return "ircord-" + strings.Join(groups, "-")
}
func normalize(key string) (string, bool) {
	key = strings.ToUpper(strings.TrimSpace(key))
	if !strings.HasPrefix(key, "IRCORD-") {
		return "", false
	}
	encoded := strings.ReplaceAll(strings.TrimPrefix(key, "IRCORD-"), "-", "")
	if len(encoded) != 30 {
		return "", false
	}
	for _, r := range encoded {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
			return "", false
		}
	}
	groups := []string{}
	for i := 0; i < 30; i += 5 {
		groups = append(groups, encoded[i:i+5])
	}
	return "ircord-" + strings.Join(groups, "-"), true
}
func keyHash(key string) string { sum := sha256.Sum256([]byte(key)); return hex.EncodeToString(sum[:]) }
func (s *Service) limit(r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ip := s.clientIP(r)
	now := time.Now()
	for key, v := range s.attempts {
		if now.After(v.until) {
			delete(s.attempts, key)
		}
	}
	if len(s.attempts) >= 10000 {
		return false
	}
	v := s.attempts[ip]
	if v.until.IsZero() {
		v.until = now.Add(time.Minute)
	}
	v.count++
	s.attempts[ip] = v
	return v.count <= 30
}
func (s *Service) Login(w http.ResponseWriter, r *http.Request, in Credentials, register bool) (LoginResult, error) {
	if !s.limit(r) {
		return LoginResult{}, ErrLimited
	}
	var account storage.Account
	var err error
	var issued string
	if register {
		if !s.options.Registration {
			return LoginResult{}, ErrRegistration
		}
		issued = newKey()
		account, err = s.store.CreateAccount(random(24), keyHash(issued))
	} else {
		normalized, ok := normalize(in.Key)
		if !ok {
			return LoginResult{}, ErrUnauthorized
		}
		account, err = s.store.AccountByKey(keyHash(normalized))
	}
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return LoginResult{}, ErrUnauthorized
		}
		return LoginResult{}, ErrUnavailable
	}
	if err = s.store.Touch(account.ID); err != nil {
		return LoginResult{}, err
	}
	account.LastSeenAt = time.Now().UTC()
	account.ExpiresAt = account.LastSeenAt.Add(storage.AccountRetention)
	token := random(32)
	sum := sha256.Sum256([]byte(token))
	sid := random(24)
	expires := time.Now().UTC().Add(s.options.SessionLifetime)
	if err = s.store.CreateSession(sid, account.ID, sum[:], expires); err != nil {
		return LoginResult{}, err
	}
	http.SetCookie(w, &http.Cookie{Name: "ircord_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.options.SecureCookies, MaxAge: int(s.options.SessionLifetime.Seconds())})
	return LoginResult{Key: issued, Account: account}, nil
}
func (s *Service) Authenticate(r *http.Request) (Principal, error) {
	cookie, err := r.Cookie("ircord_session")
	if err != nil || len(cookie.Value) > 100 {
		return Principal{}, ErrUnauthorized
	}
	sum := sha256.Sum256([]byte(cookie.Value))
	a, v, err := s.store.SessionByToken(sum[:])
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return Principal{}, ErrUnauthorized
		}
		return Principal{}, ErrUnavailable
	}
	if err = s.store.Touch(a.ID); err != nil {
		return Principal{}, err
	}
	a.LastSeenAt = time.Now().UTC()
	a.ExpiresAt = a.LastSeenAt.Add(storage.AccountRetention)
	return Principal{a, v}, nil
}
func (s *Service) Watch(ctx context.Context, p Principal) (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.store.SessionActive(p.Account.ID, p.Session.ID) {
		return nil, nil, ErrUnauthorized
	}
	child, cancel := context.WithDeadline(ctx, p.Session.ExpiresAt)
	key := random(12)
	if s.watchers[p.Session.ID] == nil {
		s.watchers[p.Session.ID] = map[string]context.CancelFunc{}
	}
	s.watchers[p.Session.ID][key] = cancel
	return child, func() {
		cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.watchers[p.Session.ID], key)
		if len(s.watchers[p.Session.ID]) == 0 {
			delete(s.watchers, p.Session.ID)
		}
	}, nil
}
func (s *Service) cancelSession(id string) {
	for _, cancel := range s.watchers[id] {
		cancel()
	}
	delete(s.watchers, id)
}
func (s *Service) Revoke(owner, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.RevokeSession(owner, id); err != nil {
		return err
	}
	s.cancelSession(id)
	return nil
}
func (s *Service) Logout(w http.ResponseWriter, p Principal) error {
	err := s.Revoke(p.Account.ID, p.Session.ID)
	http.SetCookie(w, &http.Cookie{Name: "ircord_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.options.SecureCookies, MaxAge: -1})
	return err
}
func (s *Service) Rotate(p Principal) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := newKey()
	sessions, err := s.store.Sessions(p.Account.ID)
	if err != nil {
		return "", err
	}
	if err = s.store.RotateKey(p.Account.ID, keyHash(key), p.Session.ID); err != nil {
		return "", err
	}
	for _, session := range sessions {
		if session.ID != p.Session.ID {
			s.cancelSession(session.ID)
		}
	}
	return key, nil
}
func (s *Service) Delete(owner string) error {
	if err := s.store.MarkDeleting(owner); err != nil {
		return err
	}
	sessions, err := s.store.Sessions(owner)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if err = s.Revoke(owner, session.ID); err != nil {
			return err
		}
	}
	if s.options.BeforeDelete != nil {
		if err = s.options.BeforeDelete(owner); err != nil {
			return err
		}
	}
	return s.store.DeleteAccount(owner)
}
func (s *Service) PruneExpired() error {
	owners, err := s.store.ExpiredAccounts(time.Now().Add(-storage.AccountRetention))
	if err != nil {
		return err
	}
	for _, owner := range owners {
		if err = s.Delete(owner); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Sessions(owner string) ([]storage.Session, error) { return s.store.Sessions(owner) }
func (s *Service) Touch(owner string) error                         { return s.store.Touch(owner) }
func (s *Service) RegistrationEnabled() bool                        { return s.options.Registration }
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.watchers {
		s.cancelSession(id)
	}
}

func (s *Service) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	remote = remote.Unmap()
	trusted := func(ip netip.Addr) bool {
		for _, prefix := range s.options.TrustedProxies {
			if prefix.Contains(ip) {
				return true
			}
		}
		return false
	}
	if !trusted(remote) {
		return remote.String()
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil {
			return remote.String()
		}
		ip = ip.Unmap()
		if !trusted(ip) {
			return ip.String()
		}
	}
	return remote.String()
}
