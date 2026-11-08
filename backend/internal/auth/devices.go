package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"ircord/backend/internal/storage"
)

// ErrDeviceGone is returned when a device is already signed out (or never existed).
var ErrDeviceGone = errors.New("that device is already signed out")

// RecentlySeen is how long after its last sign of life a device counts as recently connected.
const RecentlySeen = 5 * time.Minute

// Device is one signed-in browser or app installation, as shown under Devices & Sessions.
//
// A device is a session: every tab of a browser profile shares the session cookie, so tabs
// group under one device, and separate browsers or profiles are separate devices. Signing
// in again after a sign-out creates a new session, never a revived one. Web Push
// subscriptions are bound to the session, so notification routing (#84) can key on the
// same identity. Only management metadata leaves the server: the ID is a one-way digest
// of the session ID, never the session ID or token.
type Device struct {
	ID         string     `json:"id" doc:"Opaque device ID for sign-out; not a session ID or token."`
	Current    bool       `json:"current" doc:"This is the device making the request."`
	Browser    string     `json:"browser" doc:"Browser or app name from the User-Agent it signed in with; empty when unknown. Untrusted display text."`
	OS         string     `json:"os" doc:"Operating system from the same User-Agent; empty when unknown. Untrusted display text."`
	SignedInAt time.Time  `json:"signedInAt"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty" nullable:"true" doc:"Last authenticated request or live connection, written at most once a minute. Absent for sessions from before this was recorded. Background connections count, so this is not user activity."`
	ExpiresAt  time.Time  `json:"expiresAt"`
	Status     string     `json:"status" enum:"connected,recent,offline,unknown" doc:"connected: has a live event connection now; recent: seen in the last five minutes; offline: seen longer ago; unknown: never recorded."`
	Push       string     `json:"push" enum:"registered,none" doc:"Whether a Web Push subscription is registered for this device. Registered does not prove the browser will still deliver it."`
}

type DeviceList struct {
	Devices []Device `json:"devices"`
}

func deviceID(session string) string {
	sum := sha256.Sum256([]byte("ircord-device\x00" + session))
	return hex.EncodeToString(sum[:16])
}

// DeviceLabel names the browser and operating system in a User-Agent, or "" for either.
func DeviceLabel(ua string) (browser, os string) {
	has := func(s string) bool { return strings.Contains(ua, s) }
	switch {
	case has("Edg/"), has("EdgA/"), has("EdgiOS/"):
		browser = "Edge"
	case has("OPR/"):
		browser = "Opera"
	case has("Firefox/"), has("FxiOS/"):
		browser = "Firefox"
	case has("Chrome/"), has("CriOS/"), has("Chromium/"):
		browser = "Chrome"
	case has("Safari/") && has("Version/"):
		browser = "Safari"
	}
	switch {
	case has("iPhone"), has("iPod"):
		os = "iOS"
	case has("iPad"):
		os = "iPadOS"
	case has("Android"):
		os = "Android"
	case has("Windows"):
		os = "Windows"
	case has("CrOS"):
		os = "ChromeOS"
	case has("Macintosh"), has("Mac OS X"):
		os = "macOS"
	case has("Linux"):
		os = "Linux"
	}
	return browser, os
}

// devicesHub tells an account's event connections that its device list changed. Each
// subscriber holds at most one pending signal, so bursts coalesce instead of queueing.
type devicesHub struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func (h *devicesHub) subscribe(owner string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	if h.subs == nil {
		h.subs = map[string]map[chan struct{}]struct{}{}
	}
	if h.subs[owner] == nil {
		h.subs[owner] = map[chan struct{}]struct{}{}
	}
	h.subs[owner][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[owner], ch)
		if len(h.subs[owner]) == 0 {
			delete(h.subs, owner)
		}
		h.mu.Unlock()
	}
}
func (h *devicesHub) notify(owner string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[owner] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// SubscribeDevices signals when the account's devices change: a sign-in, a sign-out or a
// push registration change. Connections opening and closing (every reload) and last-seen
// ticks do not signal; the list reads them fresh whenever it is opened or refocused.
func (s *Service) SubscribeDevices(owner string) (<-chan struct{}, func()) {
	return s.devices.subscribe(owner)
}

// DevicesChanged signals the account's connections that its device list changed.
func (s *Service) DevicesChanged(owner string) { s.devices.notify(owner) }

// Devices lists the account's signed-in devices, marking the one making the request.
func (s *Service) Devices(p Principal) (DeviceList, error) {
	sessions, err := s.store.DeviceSessions(p.Account.ID)
	if err != nil {
		return DeviceList{}, ErrUnavailable
	}
	now := time.Now()
	out := DeviceList{Devices: make([]Device, 0, len(sessions))}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range sessions {
		d := Device{ID: deviceID(v.ID), Current: v.ID == p.Session.ID, SignedInAt: v.CreatedAt, ExpiresAt: v.ExpiresAt, Status: "unknown", Push: "none"}
		d.Browser, d.OS = DeviceLabel(v.UserAgent)
		if !v.LastSeenAt.IsZero() {
			seen := v.LastSeenAt
			d.LastSeenAt = &seen
			d.Status = "offline"
			if now.Sub(seen) < RecentlySeen {
				d.Status = "recent"
			}
		}
		if len(s.watchers[v.ID]) > 0 {
			d.Status = "connected"
		}
		if v.Push {
			d.Push = "registered"
		}
		out.Devices = append(out.Devices, d)
	}
	return out, nil
}

// RevokeDevice signs one of the account's devices out: its session is revoked, its event
// connections close, and its push subscriptions are removed. It reports whether that was
// the device making the request. Only the principal's own account is searched.
func (s *Service) RevokeDevice(p Principal, id string) (bool, error) {
	sessions, err := s.store.DeviceSessions(p.Account.ID)
	if err != nil {
		return false, ErrUnavailable
	}
	for _, v := range sessions {
		if deviceID(v.ID) != id {
			continue
		}
		if err = s.revoke(p.Account.ID, v.ID, s.store.RevokeLiveSession); errors.Is(err, storage.ErrNotFound) {
			return false, ErrDeviceGone
		} else if err != nil {
			return false, ErrUnavailable
		}
		return v.ID == p.Session.ID, nil
	}
	return false, ErrDeviceGone
}

// TouchSession records that the principal's session was seen (throttled), and the account.
func (s *Service) TouchSession(p Principal) error {
	if err := s.store.Touch(p.Account.ID); err != nil {
		return err
	}
	return s.store.TouchSession(p.Session.ID)
}
