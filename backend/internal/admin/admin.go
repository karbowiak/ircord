// Package admin defines the administration contract and operator credential boundary.
// Runtime initialization stays in the backend application.
package admin

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ircord/backend/internal/buildinfo"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/storage"
)

const Prefix = "/api/v1/admin"

type Status struct {
	Status        string                 `json:"status" enum:"ok,degraded"`
	Database      string                 `json:"database" enum:"available,unavailable"`
	StartedAt     time.Time              `json:"startedAt"`
	UptimeSeconds int64                  `json:"uptimeSeconds" minimum:"0"`
	Overview      *storage.AdminOverview `json:"overview,omitempty"`
	Ergo          *ergo.ErgoStatus       `json:"ergo,omitempty"`
	buildinfo.Info
}

type AdminCapabilities struct {
	Features []string `json:"features" nullable:"false"`
}
type DoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status" enum:"ok,unavailable"`
	Detail string `json:"detail"`
}
type Doctor struct {
	Status string        `json:"status" enum:"ok,degraded"`
	Checks []DoctorCheck `json:"checks" nullable:"false"`
}

// Verifier retains only a digest. A nil verifier always denies access.
type Verifier struct{ digest [32]byte }

func ValidateToken(token string) error {
	if len(token) != 64 {
		return errors.New("admin token must contain 64 hexadecimal characters (32 random bytes)")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return errors.New("admin token must contain 64 hexadecimal characters (32 random bytes)")
	}
	return nil
}

// LoadToken accepts an environment value or bounded file, never both. Empty
// configuration disables administration. Values are never included in errors.
func LoadToken(value, file string) (string, error) {
	if value != "" && file != "" {
		return "", errors.New("configure the admin token using either its value or a file, not both")
	}
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return "", errors.New("cannot open admin token file")
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return "", errors.New("admin token file must be a regular file")
		}
		b, err := io.ReadAll(io.LimitReader(f, 4097))
		if err != nil || len(b) > 4096 {
			return "", errors.New("cannot read admin token file (maximum 4096 bytes)")
		}
		value = strings.TrimSpace(string(b))
		if value == "" {
			return "", errors.New("admin token file is empty")
		}
	}
	if value != "" {
		if err := ValidateToken(value); err != nil {
			return "", err
		}
	}
	return value, nil
}

func NewVerifier(token string) (*Verifier, error) {
	if token == "" {
		return nil, nil
	}
	if err := ValidateToken(token); err != nil {
		return nil, err
	}
	return &Verifier{digest: sha256.Sum256([]byte(token))}, nil
}

func (v *Verifier) Authenticate(r *http.Request) bool {
	if v == nil || len(r.Header.Values("Authorization")) != 1 {
		return false
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) != 64 {
		return false
	}
	digest := sha256.Sum256([]byte(parts[1]))
	return subtle.ConstantTimeCompare(v.digest[:], digest[:]) == 1
}

func IsPath(path string) bool { return path == Prefix || strings.HasPrefix(path, Prefix+"/") }
