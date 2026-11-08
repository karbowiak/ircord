package ergo

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// confirmation is NickServ's "To confirm, run this command: /NS UNREGISTER <account> <code>".
var confirmation = regexp.MustCompile(`(?i)/NS UNREGISTER \S+ (\S+)\s*$`)

// Unregister removes an account on the associated server with NS UNREGISTER, which also
// unregisters the channels it founded and keeps the name reserved so it can't be registered
// again. The API can't unregister, so this resets the account's password to a random one
// (cutting off every other login), signs in over IRC with SASL as the account itself, and
// confirms the unregistration there. An account that doesn't exist (or was already
// unregistered) succeeds.
func (c *Client) Unregister(ctx context.Context, account string) error {
	if exists, err := c.AccountExists(ctx, account); err != nil || !exists {
		return err
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	password := hex.EncodeToString(secret)
	if err := c.SetPassword(ctx, account, password); errors.Is(err, ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	err := c.unregisterOverIRC(ctx, account, password)
	// Ergo disconnects the account's clients, this one included, as it unregisters, so the
	// outcome is read from the API rather than the connection.
	if exists, checkErr := c.AccountExists(ctx, account); checkErr != nil {
		return checkErr
	} else if exists {
		if err == nil {
			err = errors.New("the IRC server didn’t unregister the account")
		}
		return err
	}
	return nil
}

func (c *Client) unregisterOverIRC(ctx context.Context, account, password string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", c.server)
	if err != nil {
		return errors.New("associated IRC server is unreachable")
	}
	defer conn.Close()
	if c.ircTLS != nil {
		secure := tls.Client(conn, c.ircTLS)
		if err = secure.HandshakeContext(ctx); err != nil {
			return errors.New("associated IRC server TLS handshake failed")
		}
		conn = secure
	}
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	r := bufio.NewReader(conn)
	send := func(format string, args ...any) error {
		_, err := fmt.Fprintf(conn, format+"\r\n", args...)
		return err
	}
	nick := account
	if err = send("CAP REQ :sasl\r\nNICK %s\r\nUSER ircord 0 * :IRCord", nick); err != nil {
		return errors.New("associated IRC server connection failed")
	}
	confirmed := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if confirmed {
				return nil // disconnected while unregistering; the caller checks the result
			}
			return errors.New("associated IRC server closed the connection before unregistering")
		}
		prefix, command, params := parseLine(strings.TrimRight(line, "\r\n"))
		last := ""
		if len(params) > 0 {
			last = params[len(params)-1]
		}
		switch command {
		case "PING":
			err = send("PONG :%s", last)
		case "CAP":
			if len(params) >= 2 && params[1] == "ACK" {
				err = send("AUTHENTICATE PLAIN")
			} else if len(params) >= 2 && params[1] == "NAK" {
				return errors.New("associated IRC server doesn’t offer SASL")
			}
		case "AUTHENTICATE":
			if last == "+" {
				err = send("AUTHENTICATE %s", base64.StdEncoding.EncodeToString([]byte(account+"\x00"+account+"\x00"+password)))
			}
		case "903":
			err = send("CAP END")
		case "902", "904", "905", "906":
			return errors.New("associated IRC server refused the account’s login")
		case "432", "433":
			// The nick doesn't matter; the session is logged in as the account either way.
			suffix := make([]byte, 3)
			rand.Read(suffix)
			nick = "u" + hex.EncodeToString(suffix)
			err = send("NICK %s", nick)
		case "001":
			err = send("PRIVMSG NickServ :UNREGISTER %s", account)
		case "NOTICE", "PRIVMSG":
			if !strings.EqualFold(strings.SplitN(prefix, "!", 2)[0], "NickServ") {
				break
			}
			if m := confirmation.FindStringSubmatch(last); m != nil && !confirmed {
				confirmed = true
				err = send("PRIVMSG NickServ :UNREGISTER %s %s", account, m[1])
			} else if strings.HasPrefix(last, "Successfully unregistered") {
				send("QUIT")
				return nil
			}
		case "ERROR":
			if confirmed {
				return nil
			}
			return errors.New("associated IRC server closed the connection before unregistering")
		}
		if err != nil {
			return errors.New("associated IRC server connection failed")
		}
	}
}

// parseLine splits an IRC line into its source, command and parameters, ignoring tags.
func parseLine(line string) (prefix, command string, params []string) {
	if strings.HasPrefix(line, "@") {
		_, line, _ = strings.Cut(line, " ")
	}
	if strings.HasPrefix(line, ":") {
		prefix, line, _ = strings.Cut(line[1:], " ")
	}
	for line != "" {
		if strings.HasPrefix(line, ":") {
			params = append(params, line[1:])
			break
		}
		var param string
		param, line, _ = strings.Cut(line, " ")
		if param != "" {
			params = append(params, param)
		}
	}
	if len(params) > 0 {
		command, params = strings.ToUpper(params[0]), params[1:]
	}
	return prefix, command, params
}
