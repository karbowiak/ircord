package irc

import (
	"strconv"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

func TestTLSGrades(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	at := func(days int) *time.Time { v := now.Add(time.Duration(days) * 24 * time.Hour); return &v }
	strong := func() model.TLSInfo {
		return model.TLSInfo{Version: "TLS 1.3", Cipher: "TLS_AES_128_GCM_SHA256", Verified: true, ForwardSecrecy: true, KeyAlgorithm: "ECDSA", KeyBits: 256, SignatureAlgorithm: "ECDSA-SHA256", NotBefore: at(-30), NotAfter: at(60)}
	}
	cases := []struct {
		name  string
		edit  func(*model.TLSInfo)
		grade string
		notes int
	}{
		{"modern ECDSA", func(*model.TLSInfo) {}, "A+", 0},
		{"RSA 2048", func(i *model.TLSInfo) { i.KeyAlgorithm, i.KeyBits = "RSA", 2048 }, "A", 1},
		{"RSA 4096", func(i *model.TLSInfo) { i.KeyAlgorithm, i.KeyBits = "RSA", 4096 }, "A+", 0},
		{"TLS 1.2 AEAD with forward secrecy", func(i *model.TLSInfo) {
			i.Version, i.Cipher = "TLS 1.2", "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
		}, "A-", 1},
		{"TLS 1.2 without forward secrecy", func(i *model.TLSInfo) {
			i.Version, i.Cipher, i.ForwardSecrecy = "TLS 1.2", "TLS_RSA_WITH_AES_128_GCM_SHA256", false
		}, "B", 2},
		{"TLS 1.2 CBC", func(i *model.TLSInfo) {
			i.Version, i.Cipher = "TLS 1.2", "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA"
		}, "B-", 2},
		{"TLS 1.0", func(i *model.TLSInfo) { i.Version = "TLS 1.0" }, "D", 1},
		{"untrusted certificate", func(i *model.TLSInfo) { i.Verified = false }, "C", 1},
		{"weak RSA key", func(i *model.TLSInfo) { i.KeyAlgorithm, i.KeyBits = "RSA", 1024 }, "C", 1},
		{"SHA-1 signature", func(i *model.TLSInfo) { i.SignatureAlgorithm = "SHA1-RSA" }, "C", 1},
		{"expires soon", func(i *model.TLSInfo) { i.NotAfter = at(5) }, "B", 1},
		{"expired", func(i *model.TLSInfo) { i.NotAfter = at(-1) }, "F", 1},
		{"not yet valid", func(i *model.TLSInfo) { i.NotBefore = at(2) }, "F", 1},
	}
	for _, tc := range cases {
		info := strong()
		tc.edit(&info)
		gradeTLS(&info, now)
		if info.Grade != tc.grade || len(info.GradeNotes) != tc.notes {
			t.Errorf("%s: got %s %q, want %s with %d notes", tc.name, info.Grade, info.GradeNotes, tc.grade, tc.notes)
		}
	}
}

func TestLatencySamplesFromKeepAlive(t *testing.T) {
	manager := New(Options{})
	defer manager.Close()
	w := manager.space("alice")
	w.networks["net"] = Network{ID: "net"}
	c := &connection{owner: w, config: NetworkConfig{ID: "net"}}
	now := time.Now()
	pong := func(token string) girc.Event {
		return girc.Event{Command: "PONG", Params: []string{"irc.example", token}}
	}
	// Ours: the token is the send time.
	c.recordLatency(pong(strconv.FormatInt(now.Add(-42*time.Millisecond).UnixNano(), 10)), now)
	// Not ours, or not plausible: ignored.
	c.recordLatency(pong("LAG123"), now)
	c.recordLatency(pong(strconv.FormatInt(now.Add(-10*time.Minute).UnixNano(), 10)), now)
	c.recordLatency(pong(strconv.FormatInt(now.Add(time.Second).UnixNano(), 10)), now)
	got, err := manager.ConnectionLatency("alice", "net")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Samples) != 1 || got.Samples[0].Milliseconds < 41 || got.Samples[0].Milliseconds > 43 {
		t.Fatalf("samples = %+v", got.Samples)
	}
	if got.IntervalSeconds != 20 || got.WindowSeconds != 1800 {
		t.Fatalf("interval/window = %d/%d", got.IntervalSeconds, got.WindowSeconds)
	}
	// Samples older than the window drop out; the count stays bounded.
	old := make([]model.LatencySample, 0, maxLatencySamples+50)
	for i := range maxLatencySamples + 50 {
		old = append(old, model.LatencySample{At: now.Add(-time.Duration(maxLatencySamples+50-i) * time.Second)})
	}
	old = append([]model.LatencySample{{At: now.Add(-time.Hour)}}, old...)
	if kept := recentSamples(old, now); len(kept) != maxLatencySamples || kept[0].At.Before(now.Add(-latencyWindow)) {
		t.Fatalf("kept %d samples", len(kept))
	}
	if _, err = manager.ConnectionLatency("bob", "net"); err != ErrNotFound {
		t.Fatal("latency leaked across accounts")
	}
}
