package ergo

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeErgo is the account API plus an IRC listener speaking just enough of Ergo's SASL and
// NickServ UNREGISTER exchange.
type fakeErgo struct {
	mu       sync.Mutex
	accounts map[string]string
	retired  map[string]bool
	lines    []string
}

func (f *fakeErgo) api(w http.ResponseWriter, r *http.Request) {
	var in struct{ AccountName, Passphrase string }
	_ = json.NewDecoder(r.Body).Decode(&in)
	name := strings.ToLower(in.AccountName)
	f.mu.Lock()
	defer f.mu.Unlock()
	_, exists := f.accounts[name]
	result := map[string]any{"success": exists}
	switch r.URL.Path {
	case "/v1/ns/passwd":
		if exists {
			f.accounts[name] = in.Passphrase
		} else {
			result["errorCode"] = "ACCOUNT_DOES_NOT_EXIST"
		}
	case "/v1/ns/info":
		result["accountName"] = in.AccountName
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (f *fakeErgo) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	reply := func(format string, args ...any) { fmt.Fprintf(conn, format+"\r\n", args...) }
	nick, account := "", ""
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		f.mu.Lock()
		f.lines = append(f.lines, line)
		f.mu.Unlock()
		_, command, params := parseLine(line)
		switch command {
		case "NICK":
			nick = params[0]
		case "CAP":
			if params[0] == "REQ" {
				reply(":ergo CAP * ACK :sasl")
			} else if params[0] == "END" {
				reply(":ergo 001 %s :Welcome", nick)
				reply("PING :check")
			}
		case "AUTHENTICATE":
			if params[0] == "PLAIN" {
				reply("AUTHENTICATE +")
				continue
			}
			raw, _ := base64.StdEncoding.DecodeString(params[0])
			parts := strings.Split(string(raw), "\x00")
			f.mu.Lock()
			ok := len(parts) == 3 && f.accounts[strings.ToLower(parts[1])] == parts[2]
			f.mu.Unlock()
			if !ok {
				reply(":ergo 904 %s :SASL authentication failed", nick)
				continue
			}
			account = parts[1]
			reply(":ergo 903 %s :SASL authentication successful", nick)
		case "PRIVMSG":
			fields := strings.Fields(params[1])
			if !strings.EqualFold(params[0], "NickServ") || len(fields) < 2 || !strings.EqualFold(fields[0], "UNREGISTER") || !strings.EqualFold(fields[1], account) {
				continue
			}
			if len(fields) == 2 {
				reply(":NickServ!NickServ@localhost NOTICE %s :\x02Note that an unregistered account name remains reserved and cannot be re-registered.\x02", nick)
				reply(":NickServ!NickServ@localhost NOTICE %s :To confirm, run this command: /NS UNREGISTER %s c0de", nick, account)
				continue
			}
			if fields[2] != "c0de" {
				continue
			}
			// Like Ergo, the account's clients are killed before the reply is flushed.
			f.mu.Lock()
			delete(f.accounts, strings.ToLower(account))
			f.retired[strings.ToLower(account)] = true
			f.mu.Unlock()
			reply("ERROR :Closing Link: unregistered")
			return
		}
	}
}

func TestUnregister(t *testing.T) {
	fake := &fakeErgo{accounts: map[string]string{"alice": "her-password", "bob": "x"}, retired: map[string]bool{}}
	api := httptest.NewServer(http.HandlerFunc(fake.api))
	defer api.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go fake.serve(conn)
		}
	}()
	c, err := New(Options{Server: listener.Addr().String(), URL: api.URL, Token: strings.Repeat("ab", 32), IRCPlaintext: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	if err = c.Unregister(ctx, "Alice"); err != nil {
		t.Fatal(err)
	}
	if _, ok := fake.accounts["alice"]; ok || !fake.retired["alice"] {
		t.Fatal("account not unregistered", fake.accounts)
	}
	joined := strings.Join(fake.lines, "\n")
	if strings.Contains(joined, "her-password") || !strings.Contains(joined, "PONG :check") {
		t.Fatal("unexpected exchange", joined)
	}
	// Unregistering again, or a name that never existed, is a no-op.
	for _, name := range []string{"alice", "nobody"} {
		if err = c.Unregister(ctx, name); err != nil {
			t.Fatal(name, err)
		}
	}
	if _, ok := fake.accounts["bob"]; !ok {
		t.Fatal("another account was touched")
	}
	// A server that never confirms leaves the account registered and reports it.
	listener.Close()
	if err = c.Unregister(ctx, "bob"); err == nil {
		t.Fatal("unreachable IRC server reported success")
	}
}

// TestUnregisterAgainstErgo runs against a disposable Ergo with the API enabled:
// IRCORD_TEST_ERGO_SERVER (plaintext IRC host:port), IRCORD_TEST_ERGO_API_URL and
// IRCORD_TEST_ERGO_API_TOKEN.
func TestUnregisterAgainstErgo(t *testing.T) {
	server, url, token := os.Getenv("IRCORD_TEST_ERGO_SERVER"), os.Getenv("IRCORD_TEST_ERGO_API_URL"), os.Getenv("IRCORD_TEST_ERGO_API_TOKEN")
	if server == "" || url == "" || token == "" {
		t.Skip("set IRCORD_TEST_ERGO_SERVER, IRCORD_TEST_ERGO_API_URL and IRCORD_TEST_ERGO_API_TOKEN")
	}
	c, err := New(Options{Server: server, URL: url, Token: token, IRCPlaintext: true, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	name := fmt.Sprintf("unreg%d", time.Now().UnixNano()%1e9)
	if err = c.Register(ctx, name, "first-password"); err != nil {
		t.Fatal(err)
	}
	if err = c.Unregister(ctx, name); err != nil {
		t.Fatal(err)
	}
	if exists, err := c.AccountExists(ctx, name); err != nil || exists {
		t.Fatal("account still registered", err)
	}
	if ok, _ := c.CheckPassword(ctx, name, "first-password"); ok {
		t.Fatal("old password still accepted")
	}
	if err = c.Register(ctx, strings.ToUpper(name), "second-password"); !errors.Is(err, ErrAccountExists) {
		t.Fatal("unregistered name could be registered again", err)
	}
	if err = c.Unregister(ctx, name); err != nil {
		t.Fatal("unregistering again failed", err)
	}
}
