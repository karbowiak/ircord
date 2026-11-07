package irc

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ParseNetworkPasswords reads operator-owned TLS endpoint defaults. These are
// connection credentials, never public preset data or HTTP response fields.
func ParseNetworkPasswords(raw []byte) (map[string]string, error) {
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("invalid network password configuration")
	}
	result := map[string]string{}
	for endpoint, password := range values {
		host, port, err := net.SplitHostPort(endpoint)
		n, portErr := strconv.Atoi(port)
		if err != nil || portErr != nil || host == "" || strings.ContainsAny(host, " /\\@?#\r\n") || n < 1 || n > 65535 || password == "" || !clean(password, 512) {
			return nil, fmt.Errorf("invalid network password configuration")
		}
		result[net.JoinHostPort(strings.ToLower(host), strconv.Itoa(n))] = password
	}
	return result, nil
}

func (m *Manager) configuredPassword(cfg NetworkConfig) NetworkConfig {
	if !cfg.TLS || cfg.Auth == nil || cfg.Auth.Method != "server-password" || cfg.Auth.Password != nil {
		return cfg
	}
	password := m.options.NetworkPasswords[net.JoinHostPort(strings.ToLower(cfg.Host), strconv.Itoa(cfg.Port))]
	if password == "" {
		return cfg
	}
	cfg = clone(cfg)
	cfg.Auth.Password = &password
	return cfg
}
