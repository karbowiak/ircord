package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"ircord/backend/internal/storage"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

var ErrUnauthorized = errors.New("that username and password don’t match")
var ErrSession = errors.New("sign in to continue")
var ErrUnavailable = errors.New("account storage is unavailable")
var ErrRegistration = errors.New("registration is disabled")
var ErrRegistrationKey = errors.New("registration key is missing or incorrect")
var ErrLimited = errors.New("too many authentication attempts; try again shortly")
var ErrUsernameTaken = errors.New("that username is already taken")
var ErrWrongPassword = errors.New("your current password is wrong")

// InvalidError is a username or password that doesn't meet the rules; its text is shown as is.
type InvalidError struct{ Reason string }

func (e InvalidError) Error() string { return e.Reason }

type Credentials struct {
	Username string `json:"username" minLength:"1" maxLength:"64"`
	Password string `json:"password" minLength:"1" maxLength:"256" writeOnly:"true"`
}
type SignupCredentials struct {
	Username        string `json:"username" minLength:"1" maxLength:"64"`
	DisplayName     string `json:"displayName,omitempty" maxLength:"200" doc:"How the account appears in chat; optional, and changeable later."`
	Password        string `json:"password" minLength:"1" maxLength:"256" writeOnly:"true"`
	RegistrationKey string `json:"registrationKey,omitempty" maxLength:"256" writeOnly:"true"`
}
type PasswordChange struct {
	Current  string `json:"current" minLength:"1" maxLength:"256" writeOnly:"true"`
	Password string `json:"password" minLength:"1" maxLength:"256" writeOnly:"true"`
}
type UsernameCheck struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// Accounts is the associated IRC server's account system (Ergo), when one is configured.
// Signup creates the account there too, and password changes follow.
type Accounts interface {
	Register(ctx context.Context, account, passphrase string) error
	SetPassword(ctx context.Context, account, passphrase string) error
	AccountExists(ctx context.Context, name string) (bool, error)
	CheckPassword(ctx context.Context, name, password string) (bool, error)
	// Unregister removes an account and keeps its name reserved. Unregistering an account
	// that doesn't exist (any more) succeeds.
	Unregister(ctx context.Context, account string) error
}
type RegistrationPolicy struct {
	RegistrationEnabled     bool `json:"registrationEnabled"`
	RegistrationKeyRequired bool `json:"registrationKeyRequired"`
}
type Principal struct {
	Account storage.Account `json:"account"`
	Session storage.Session `json:"-"`
}
type LoginResult struct {
	Account storage.Account `json:"account"`
}
type Options struct {
	TrustedProxies         []netip.Prefix
	SecureCookies          bool
	Registration           bool
	RegistrationKey        string
	RequireRegistrationKey bool
	SessionLifetime        time.Duration
	BeforeDelete           func(string) error
	// Accounts, when set, makes every IRCord account an account on the associated IRC server.
	Accounts Accounts
	// Provision sets up a new account's connection to the associated IRC server, and
	// PasswordChanged updates its saved login. Both are optional.
	Provision       func(owner, username, password string) error
	PasswordChanged func(owner, password string) error
	// SetDisplayName applies the display name chosen at signup.
	SetDisplayName func(owner, name string) error
	// SessionEnded is told when a session is signed out or revoked. It is called with the
	// service's lock held, so it must not call back into the service.
	SessionEnded func(owner, session string)
	// SignedOut is told when the account signed itself out of its last device, and SignedIn
	// whenever a sign-in starts a session. Both are optional and called without the lock.
	SignedOut func(owner string) error
	SignedIn  func(owner string) error
}
type Service struct {
	registrationKeyHash     [32]byte
	registrationKeySet      bool
	registrationKeyRequired bool
	store                   *storage.Store
	options                 Options
	mu                      sync.Mutex
	credentialLocks         [64]sync.Mutex
	unregisterMu            sync.Mutex
	attempts                map[string]attempt
	watchers                map[string]map[string]context.CancelFunc
	devices                 devicesHub

	// presenceLocks keep an account's "last device signed out" check and a new sign-in in
	// order, so a sign-in can't land between the check and SignedOut.
	presenceLocks [64]sync.Mutex
}
type attempt struct {
	count int
	until time.Time
}

func New(store *storage.Store, options Options) *Service {
	if options.SessionLifetime <= 0 {
		options.SessionLifetime = 90 * 24 * time.Hour
	}
	key := options.RegistrationKey
	options.RegistrationKey = ""
	return &Service{registrationKeyHash: sha256.Sum256([]byte(key)), registrationKeySet: key != "", registrationKeyRequired: options.RequireRegistrationKey || key != "", store: store, options: options, attempts: map[string]attempt{}, watchers: map[string]map[string]context.CancelFunc{}}
}
func (s *Service) RegistrationPolicy() RegistrationPolicy {
	return RegistrationPolicy{RegistrationEnabled: s.options.Registration, RegistrationKeyRequired: s.registrationKeyRequired}
}
func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

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

// Signup creates an account: on the associated IRC server first (when configured), then here,
// then its connection to that server. It signs the new account in.
func (s *Service) Signup(w http.ResponseWriter, r *http.Request, in SignupCredentials) (LoginResult, error) {
	if !s.limit(r) {
		return LoginResult{}, ErrLimited
	}
	if !s.options.Registration {
		return LoginResult{}, ErrRegistration
	}
	if s.registrationKeyRequired {
		provided := sha256.Sum256([]byte(in.RegistrationKey))
		if !s.registrationKeySet || subtle.ConstantTimeCompare(provided[:], s.registrationKeyHash[:]) != 1 {
			return LoginResult{}, ErrRegistrationKey
		}
	}
	username := strings.TrimSpace(in.Username)
	if reason := UsernameProblem(username); reason != "" {
		return LoginResult{}, InvalidError{reason}
	}
	if reason := PasswordProblem(in.Password); reason != "" {
		return LoginResult{}, InvalidError{reason}
	}
	displayName := strings.TrimSpace(in.DisplayName)
	if reason := DisplayNameProblem(displayName); reason != "" {
		return LoginResult{}, InvalidError{reason}
	}
	lock := s.credentialLock(username)
	lock.Lock()
	defer lock.Unlock()
	op, err := s.loadOperation(username)
	if err != nil {
		return LoginResult{}, ErrUnavailable
	}
	if op != nil {
		if op.Kind != "signup" || !verifyPassword(in.Password, op.Hash) {
			return LoginResult{}, ErrUsernameTaken
		}
		op.DisplayName = displayName
		if err := s.saveOperation(op); err != nil {
			return LoginResult{}, ErrUnavailable
		}
	} else {
		if taken, err := s.store.UsernameTaken(username); err != nil {
			return LoginResult{}, ErrUnavailable
		} else if taken {
			return LoginResult{}, ErrUsernameTaken
		}
		hash, err := hashPassword(in.Password)
		if err != nil {
			return LoginResult{}, ErrUnavailable
		}
		op = &credentialOperation{Kind: "signup", Owner: random(24), Username: username, Hash: hash, Password: in.Password, DisplayName: displayName}
		if err := s.saveOperation(op); err != nil {
			return LoginResult{}, ErrUnavailable
		}
	}
	if err := s.finishOperation(r.Context(), op); err != nil {
		return LoginResult{}, err
	}
	account, hash, err := s.store.AccountByUsername(username)
	if err != nil {
		return LoginResult{}, ErrUnavailable
	}
	result := LoginResult{account}
	return result, s.startSession(w, r, result, hash)
}

// Login signs in with a username and password.
func (s *Service) Login(w http.ResponseWriter, r *http.Request, in Credentials) (LoginResult, error) {
	if !s.limit(r) {
		return LoginResult{}, ErrLimited
	}
	username := strings.TrimSpace(in.Username)
	lock := s.credentialLock(username)
	lock.Lock()
	defer lock.Unlock()
	if op, err := s.loadOperation(username); err != nil {
		return LoginResult{}, ErrUnavailable
	} else if op != nil {
		if op.Kind == "signup" && !verifyPassword(in.Password, op.Hash) {
			return LoginResult{}, ErrUnauthorized
		}
		if err := s.finishOperation(r.Context(), op); err != nil {
			return LoginResult{}, err
		}
	}
	account, hash, err := s.store.AccountByUsername(username)
	if errors.Is(err, storage.ErrNotFound) {
		// Same work as a real check, so timing doesn't reveal which usernames exist.
		_ = verifyPassword(in.Password, dummyHash)
		return LoginResult{}, ErrUnauthorized
	} else if err != nil {
		return LoginResult{}, ErrUnavailable
	}
	if !verifyPassword(in.Password, hash) {
		return LoginResult{}, ErrUnauthorized
	}
	if err = s.store.Touch(account.ID); err != nil {
		return LoginResult{}, ErrUnavailable
	}
	account.LastSeenAt = time.Now().UTC()
	account.ExpiresAt = account.LastSeenAt.Add(storage.AccountRetention)
	result := LoginResult{account}
	return result, s.startSession(w, r, result, hash)
}

func (s *Service) startSession(w http.ResponseWriter, r *http.Request, result LoginResult, passwordHash string) error {
	token := random(32)
	sum := sha256.Sum256([]byte(token))
	sid := random(24)
	expires := time.Now().UTC().Add(s.options.SessionLifetime)
	lock := s.presenceLock(result.Account.ID)
	lock.Lock()
	defer lock.Unlock()
	if err := s.store.CreateVerifiedSession(sid, result.Account.ID, sum[:], expires, passwordHash, r.UserAgent()); err != nil {
		return ErrUnavailable
	}
	s.devices.notify(result.Account.ID)
	http.SetCookie(w, &http.Cookie{Name: "ircord_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.options.SecureCookies, MaxAge: int(s.options.SessionLifetime.Seconds())})
	if s.options.SignedIn != nil {
		if err := s.options.SignedIn(result.Account.ID); err != nil {
			slog.Error("Reconnecting networks after sign-in failed", "error", err)
		}
	}
	return nil
}

// signedOut tells SignedOut when the account just signed itself out of its last device.
// Sessions that expire or a password reset revokes are not a deliberate sign-out.
func (s *Service) signedOut(owner string) {
	if s.options.SignedOut == nil {
		return
	}
	// A sign-in either made its session before this check, which then finds it, or runs
	// its SignedIn after SignedOut and reconnects.
	lock := s.presenceLock(owner)
	lock.Lock()
	defer lock.Unlock()
	sessions, err := s.store.Sessions(owner)
	if err != nil {
		slog.Error("Checking for other signed-in devices failed", "error", err)
		return
	}
	for _, v := range sessions {
		if !v.Revoked {
			return
		}
	}
	if err = s.options.SignedOut(owner); err != nil {
		slog.Error("Disconnecting networks after sign-out failed", "error", err)
	}
}

func (s *Service) presenceLock(owner string) *sync.Mutex {
	var h uint32
	for _, b := range []byte(owner) {
		h = h*31 + uint32(b)
	}
	return &s.presenceLocks[h%uint32(len(s.presenceLocks))]
}

// CheckUsername says whether a username can be used for signup.
func (s *Service) CheckUsername(ctx context.Context, name string) (UsernameCheck, error) {
	name = strings.TrimSpace(name)
	if reason := UsernameProblem(name); reason != "" {
		return UsernameCheck{Reason: reason}, nil
	}
	taken, err := s.store.UsernameTaken(name)
	if err == nil && !taken && s.options.Accounts != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		taken, err = s.options.Accounts.AccountExists(ctx, name)
		cancel()
	}
	if err != nil {
		return UsernameCheck{}, ErrUnavailable
	}
	if taken {
		return UsernameCheck{Reason: name + " is already taken."}, nil
	}
	return UsernameCheck{Available: true}, nil
}

// ChangePassword checks the current password, then changes it on the associated IRC server
// and here, and signs out every other session.
func (s *Service) ChangePassword(ctx context.Context, p Principal, in PasswordChange) error {
	lock := s.credentialLock(p.Account.Username)
	lock.Lock()
	defer lock.Unlock()
	if op, err := s.loadOperation(p.Account.Username); err != nil {
		return ErrUnavailable
	} else if op != nil {
		if err := s.finishOperation(ctx, op); err != nil {
			return err
		}
	}
	if !s.store.SessionActive(p.Account.ID, p.Session.ID) {
		return ErrSession
	}
	hash, err := s.store.PasswordHash(p.Account.ID)
	if err != nil {
		return ErrUnavailable
	}
	if !verifyPassword(in.Current, hash) {
		return ErrWrongPassword
	}
	if reason := PasswordProblem(in.Password); reason != "" {
		return InvalidError{reason}
	}
	next, err := hashPassword(in.Password)
	if err != nil {
		return ErrUnavailable
	}
	op := &credentialOperation{Kind: "password", Owner: p.Account.ID, Username: p.Account.Username, Hash: next, Password: in.Password, KeepSession: p.Session.ID}
	if err := s.saveOperation(op); err != nil {
		return ErrUnavailable
	}
	return s.finishOperation(ctx, op)
}

func (s *Service) Authenticate(r *http.Request) (Principal, error) {
	cookie, err := r.Cookie("ircord_session")
	if err != nil || len(cookie.Value) > 100 {
		return Principal{}, ErrSession
	}
	sum := sha256.Sum256([]byte(cookie.Value))
	a, v, err := s.store.SessionByToken(sum[:])
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return Principal{}, ErrSession
		}
		return Principal{}, ErrUnavailable
	}
	// Both writes are throttled; skip them entirely while the stored times are fresh, so most
	// requests take no write lock.
	if time.Since(a.LastSeenAt) >= time.Minute {
		if err = s.store.Touch(a.ID); err != nil {
			return Principal{}, err
		}
	}
	if time.Since(v.LastSeenAt) >= storage.SessionTouchInterval {
		if err = s.store.TouchSession(v.ID); err != nil {
			return Principal{}, err
		}
	}
	a.LastSeenAt = time.Now().UTC()
	a.ExpiresAt = a.LastSeenAt.Add(storage.AccountRetention)
	return Principal{a, v}, nil
}
func (s *Service) Watch(ctx context.Context, p Principal) (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.store.SessionActive(p.Account.ID, p.Session.ID) {
		return nil, nil, ErrSession
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
func (s *Service) sessionEnded(owner, id string) {
	if s.options.SessionEnded != nil {
		s.options.SessionEnded(owner, id)
	}
}
func (s *Service) cancelSession(id string) {
	for _, cancel := range s.watchers[id] {
		cancel()
	}
	delete(s.watchers, id)
}
func (s *Service) Revoke(owner, id string) error {
	return s.revoke(owner, id, s.store.RevokeSession)
}

// revoke revokes a session and closes its live connections. Whatever the revoke returned,
// a session that is no longer authorized afterwards has its connections cut: a revocation
// must never leave a socket open (and an unreadable session fails closed).
func (s *Service) revoke(owner, id string, revoke func(owner, id string) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := revoke(owner, id)
	if err == nil || !s.store.SessionActive(owner, id) {
		s.cancelSession(id)
		s.sessionEnded(owner, id)
		s.devices.notify(owner)
	}
	return err
}

// SessionActive reports whether the principal's session is still authorized.
func (s *Service) SessionActive(p Principal) bool {
	return s.store.SessionActive(p.Account.ID, p.Session.ID)
}

// ClearSessionCookie removes the session cookie with the attributes it was set with.
func (s *Service) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "ircord_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.options.SecureCookies, MaxAge: -1})
}
func (s *Service) Logout(w http.ResponseWriter, p Principal) error {
	err := s.Revoke(p.Account.ID, p.Session.ID)
	s.ClearSessionCookie(w)
	if err == nil {
		s.signedOut(p.Account.ID)
	}
	return err
}
func (s *Service) Delete(owner string) error {
	var username string
	if err := s.store.DB.QueryRow("SELECT COALESCE(username,'') FROM accounts WHERE id=?", owner).Scan(&username); err != nil {
		return err
	}
	lock := s.credentialLock(username)
	lock.Lock()
	defer lock.Unlock()
	if err := s.store.DeleteCredentialOperation(username); err != nil {
		return err
	}
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
	if err = s.store.DeleteAccount(owner, s.options.Accounts != nil && username != ""); err != nil {
		return err
	}
	if s.options.Accounts != nil && username != "" {
		s.unregister(context.Background(), username)
	}
	return nil
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
