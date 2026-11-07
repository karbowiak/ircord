// Package credentials reads operator credentials without exposing their values.
package credentials

import (
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
)

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
