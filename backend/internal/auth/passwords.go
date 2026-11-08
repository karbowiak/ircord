package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"ircord/backend/internal/ergo"
)

// Password hashes are argon2id in the PHC string format, with the OWASP baseline parameters.
const (
	argonTime    = 2
	argonMemory  = 19 * 1024
	argonThreads = 1
	argonKeyLen  = 32
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func verifyPassword(password, encoded string) bool {
	var version, memory, time int
	var threads uint8
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil || memory < 1 || memory > 1<<20 || time < 1 || time > 10 || threads < 1 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 || len(want) > 64 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(time), uint32(memory), threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is checked against when a username doesn't exist, so a failed login costs the same.
var dummyHash, _ = hashPassword("ircord-no-such-account")

// Usernames are also IRC nicks and Ergo account names.
var usernameSyntax = regexp.MustCompile(`^[A-Za-z\[\]\\` + "`" + `_^{}|][A-Za-z0-9\[\]\\` + "`" + `_^{}|-]*$`)
var reservedUsernames = map[string]bool{"chanserv": true, "nickserv": true, "histserv": true, "hostserv": true, "operserv": true, "botserv": true, "memoserv": true, "global": true, "ircord": true, "admin": true, "root": true}

const MaxUsername = 30

// UsernameProblem explains why a username can't be used, or returns "".
func UsernameProblem(name string) string {
	switch {
	case name == "":
		return "Enter a username."
	case strings.ContainsAny(name, " \t"):
		return "Usernames can’t contain spaces. Your display name can."
	case !usernameSyntax.MatchString(name):
		return "Use letters, numbers or IRC nickname symbols, starting with a letter or symbol."
	case len(name) > MaxUsername:
		return fmt.Sprintf("Use %d characters or fewer.", MaxUsername)
	case reservedUsernames[strings.ToLower(name)]:
		return name + " is reserved."
	}
	return ""
}

// PasswordProblem explains why a password can't be used, or returns "".
func PasswordProblem(password string) string {
	switch n := utf8.RuneCountInString(password); {
	case n < 8:
		return "Use at least 8 characters for your password."
	case n > 128:
		return "Use 128 characters or fewer for your password."
	case strings.IndexFunc(password, func(r rune) bool { return unicode.IsSpace(r) || r == 0 }) >= 0:
		// Ergo refuses passphrases with whitespace, and IRC clients send them space-separated.
		return "Your password can’t contain spaces."
	}
	return ""
}

// accountsError maps an associated-server failure to what the user should see.
func accountsError(err error) error {
	switch {
	case errors.Is(err, ergo.ErrAccountExists):
		return ErrUsernameTaken
	case errors.Is(err, ergo.ErrInvalidPassphrase):
		return InvalidError{"The IRC server didn’t accept that password. Try a different one."}
	case errors.Is(err, ergo.ErrAccountRefused):
		return InvalidError{"The IRC server didn’t accept that username. Try a different one."}
	}
	return ErrNetworkUnavailable
}

// ErrNetworkUnavailable means the associated IRC server couldn't be reached to create or update
// the account; nothing was changed.
var ErrNetworkUnavailable = errors.New("the IRC server couldn’t be reached; try again shortly")
