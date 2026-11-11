package irc

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

// This diagnostic uses a separate Docker container, never IRCORD_ERGO_ADDR.
func TestProfileRestartDiagnostic(t *testing.T) {
	for _, imported := range []bool{false, true} {
		t.Run(fmt.Sprintf("imported=%t", imported), func(t *testing.T) {
			host, dockerPort, _ := isolatedPartyErgo(t)
			raw, err := exec.Command("docker", "ps", "-q", "--filter", "name=ircord-party-", "--filter", "publish="+strconv.Itoa(dockerPort)).Output()
			if err != nil || len(strings.Fields(string(raw))) != 1 {
				t.Fatalf("cannot identify dedicated container: %v %q", err, raw)
			}
			container := strings.TrimSpace(string(raw))
			// Docker assigns a new random port on restart. Keep the IRC endpoint stable.
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { listener.Close() })
			var destination atomic.Int64
			destination.Store(int64(dockerPort))
			go func() {
				for {
					incoming, err := listener.Accept()
					if err != nil {
						return
					}
					go func() {
						defer incoming.Close()
						upstream, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(destination.Load()))))
						if err != nil {
							return
						}
						defer upstream.Close()
						go func() { io.Copy(upstream, incoming); upstream.Close() }()
						io.Copy(incoming, upstream)
					}()
				}
			}()
			port := listener.Addr().(*net.TCPAddr).Port
			s, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.Close() })
			for _, owner := range []string{"alice", "bob"} {
				if _, err := s.CreateAccount(owner, owner+"-diagnostic"); err != nil {
					t.Fatal(err)
				}
			}
			suffix := id()[:8]
			f := &metadataFixture{t: t, host: host, port: port, store: s, alice: "diagA" + suffix, bob: "diagB" + suffix, room: "#diag" + suffix}
			f.server = serverMetadata(t, host, port)
			password := alwaysOnAccount(t, host, port, f.alice)
			f.aliceSide = New(Options{Store: s, AllowPrivate: true})
			f.bobSide = New(Options{Store: s, AllowPrivate: true})
			t.Cleanup(func() { f.aliceSide.Close(); f.bobSide.Close() })
			f.connect(f.bobSide, "bob", f.config(f.bob, ""), true)
			f.connect(f.aliceSide, "alice", f.config(f.alice, password), true)
			profileChecked(t, f.aliceSide, "alice")
			if _, err := f.aliceSide.SetNetworkProfile("alice", "net", Profile{AvatarURL: profileName(testAvatar), Bio: "diagnostic bio"}); err != nil {
				t.Fatal(err)
			}
			flushed(t, f.aliceSide, "alice")
			eventually(t, func() bool {
				return f.server(f.alice, "avatar", testAvatar) && f.server(f.alice, "bio", "diagnostic bio") && f.peer().AvatarURL == testAvatar && f.peer().Bio == "diagnostic bio"
			})
			if imported {
				own, _ := f.aliceSide.NetworkProfile("alice", "net")
				var n Network
				for _, candidate := range f.aliceSide.Snapshot("alice").Networks {
					if candidate.ID == "net" {
						n = candidate
					}
				}
				f.aliceSide.Close()
				if _, err := s.Commit("alice", "profile.updated", own); err != nil {
					t.Fatal(err)
				}
				n.Profile = nil
				if _, err := s.Commit("alice", "network.updated", n); err != nil {
					t.Fatal(err)
				}
				f.aliceSide = New(Options{Store: s, AllowPrivate: true})
				if err := f.aliceSide.Restore(); err != nil {
					t.Fatal(err)
				}
				profileChecked(t, f.aliceSide, "alice")
			}
			imports, err := s.ProfileImports("alice", "net")
			if err != nil {
				t.Fatal(err)
			}
			c, err := f.aliceSide.connection("alice", "net")
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("before restart: imported=%t ownerAvatar=true ownerBio=true peerAvatar=true peerBio=true serverAvatar=true serverBio=true flags=%v published=%v", imported, imports, c.publishedProfile())
			if imported && (imports["avatar"] || imports["bio"]) {
				t.Fatal("matching server values did not release imported flags")
			}
			f.aliceSide.Close()
			f.bobSide.Close()
			if out, err := exec.Command("docker", "restart", "--time", "2", container).CombinedOutput(); err != nil {
				t.Fatalf("restart dedicated fixture: %v %s", err, out)
			}
			portRaw, err := exec.Command("docker", "port", container, "6667").Output()
			if err != nil {
				t.Fatal(err)
			}
			_, portText, err := net.SplitHostPort(strings.TrimSpace(string(portRaw)))
			if err != nil {
				t.Fatal(err)
			}
			newPort, err := strconv.Atoi(portText)
			if err != nil {
				t.Fatal(err)
			}
			destination.Store(int64(newPort))
			deadline := time.Now().Add(20 * time.Second)
			for {
				conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(newPort)), time.Second)
				if err == nil {
					conn.Close()
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("restarted server did not listen", err)
				}
				time.Sleep(100 * time.Millisecond)
			}
			f.server = serverMetadata(t, host, port)
			f.aliceSide = New(Options{Store: s, AllowPrivate: true})
			f.bobSide = New(Options{Store: s, AllowPrivate: true})
			if err := f.bobSide.Restore(); err != nil {
				t.Fatal(err)
			}
			if err := f.aliceSide.Restore(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := f.aliceSide.WaitConnected(ctx, "alice", "net"); err != nil {
				t.Fatal(err)
			}
			profileChecked(t, f.aliceSide, "alice")
			eventually(t, func() bool { return f.peer().Nick == f.alice })
			// Allow the peer's JOIN-triggered metadata sync to finish before observation.
			flushed(t, f.bobSide, "bob")
			own, _ := f.aliceSide.NetworkProfile("alice", "net")
			peer := f.peer()
			serverAvatar := f.server(f.alice, "avatar", testAvatar)
			serverBio := f.server(f.alice, "bio", "diagnostic bio")
			missingAvatar := f.server(f.alice, "avatar", "")
			missingBio := f.server(f.alice, "bio", "")
			imports, _ = s.ProfileImports("alice", "net")
			t.Logf("after restart: imported=%t ownerAvatar=%t ownerBio=%t peerAvatar=%t peerBio=%t serverAvatar=%t serverBio=%t keyNotSetAvatar=%t keyNotSetBio=%t flags=%v", imported, own.AvatarURL == testAvatar, own.Bio == "diagnostic bio", peer.AvatarURL == testAvatar, peer.Bio == "diagnostic bio", serverAvatar, serverBio, missingAvatar, missingBio, imports)
			if !serverAvatar || !serverBio {
				t.Fatal("unexpected metadata loss on ordinary server restart")
			}

			// Change only these disposable upstream keys while both backends are offline.
			f.aliceSide.Close()
			f.bobSide.Close()
			if imported {
				// Simulate a backend upgraded after the old code recorded the matching
				// values but before it could release these stale import flags.
				imports["avatar"], imports["bio"] = true, true
				if err := s.SaveProfileImports("alice", "net", imports); err != nil {
					t.Fatal(err)
				}
			}
			clearer := girc.New(girc.Config{Server: host, Port: port, Nick: f.alice, User: "diag", Name: "diag", AllowFlood: true, SASL: &girc.SASLPlain{User: f.alice, Pass: password}, SupportedCaps: map[string][]string{"draft/metadata-3": nil}})
			ready := make(chan struct{})
			clearer.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) { close(ready) })
			go func() { _ = clearer.Connect() }()
			select {
			case <-ready:
			case <-time.After(10 * time.Second):
				t.Fatal("clearer did not connect")
			}
			clearer.Send(&girc.Event{Command: "METADATA", Params: []string{"*", "SET", "avatar"}})
			clearer.Send(&girc.Event{Command: "METADATA", Params: []string{"*", "SET", "bio"}})
			eventually(t, func() bool { return f.server(f.alice, "avatar", "") && f.server(f.alice, "bio", "") })
			clearer.Close()
			f.aliceSide = New(Options{Store: s, AllowPrivate: true})
			f.bobSide = New(Options{Store: s, AllowPrivate: true})
			if err := f.bobSide.Restore(); err != nil {
				t.Fatal(err)
			}
			if err := f.aliceSide.Restore(); err != nil {
				t.Fatal(err)
			}
			profileChecked(t, f.aliceSide, "alice")
			eventually(t, func() bool { p := f.peer(); return p.AvatarURL == testAvatar && p.Bio == "diagnostic bio" })
			own, _ = f.aliceSide.NetworkProfile("alice", "net")
			peer = f.peer()
			serverAvatar = f.server(f.alice, "avatar", testAvatar)
			serverBio = f.server(f.alice, "bio", "diagnostic bio")
			missingAvatar = f.server(f.alice, "avatar", "")
			missingBio = f.server(f.alice, "bio", "")
			imports, _ = s.ProfileImports("alice", "net")
			t.Logf("after offline upstream clear and reconnect: imported=%t ownerAvatar=%t ownerBio=%t peerAvatar=%t peerBio=%t serverAvatar=%t serverBio=%t keyNotSetAvatar=%t keyNotSetBio=%t flags=%v", imported, own.AvatarURL == testAvatar, own.Bio == "diagnostic bio", peer.AvatarURL == testAvatar, peer.Bio == "diagnostic bio", serverAvatar, serverBio, missingAvatar, missingBio, imports)
			if !serverAvatar || !serverBio || own.AvatarURL != testAvatar || own.Bio != "diagnostic bio" || missingAvatar || missingBio {
				t.Fatal("established profile was not republished")
			}
			if imported && (imports["avatar"] || imports["bio"]) {
				t.Fatal("matching publication history did not release stale import flags")
			}
			{
				live := girc.New(girc.Config{Server: host, Port: port, Nick: f.alice, User: "diag", Name: "diag", AllowFlood: true, SASL: &girc.SASLPlain{User: f.alice, Pass: password}, SupportedCaps: map[string][]string{"draft/metadata-3": nil}})
				liveReady := make(chan struct{})
				live.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) { close(liveReady) })
				go func() { _ = live.Connect() }()
				select {
				case <-liveReady:
				case <-time.After(10 * time.Second):
					t.Fatal("live clearer did not connect")
				}
				live.Send(&girc.Event{Command: "METADATA", Params: []string{"*", "SET", "avatar"}})
				live.Send(&girc.Event{Command: "METADATA", Params: []string{"*", "SET", "bio"}})
				eventually(t, func() bool {
					p, _ := f.aliceSide.NetworkProfile("alice", "net")
					return p.AvatarURL == "" && p.Bio == ""
				})
				eventually(t, func() bool { p := f.peer(); return p.AvatarURL == "" && p.Bio == "" })
				t.Log("live remote clear: ownerAvatar=false ownerBio=false peerAvatar=false peerBio=false")
				live.Close()
			}
		})
	}
}
