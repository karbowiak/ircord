package irc

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"go.yaml.in/yaml/v3"
)

// NetworkPreset is public connection guidance; gateway configuration is private.
type NetworkPreset struct {
	Name             string   `json:"name"`
	Host             string   `json:"host"`
	Port             int      `json:"port"`
	TLS              bool     `json:"tls"`
	AuthMethod       string   `json:"authMethod" enum:"none,server-password,sasl-plain,nickserv"`
	AutoJoin         []string `json:"autoJoin" nullable:"false"`
	Color            string   `json:"color" enum:"lavender,mint,peach,blue,yellow"`
	Initial          string   `json:"initial"`
	PasswordProvided bool     `json:"passwordProvided"`
}

type webIRCGateway struct {
	Password string
	Gateway  string
}

type ServerList struct {
	Configured bool
	Presets    []NetworkPreset
	gateways   map[string]webIRCGateway
}

type serverListFile struct {
	Servers []struct {
		Name       string   `yaml:"name"`
		Host       string   `yaml:"host"`
		Port       *int     `yaml:"port"`
		TLS        *bool    `yaml:"tls"`
		AuthMethod string   `yaml:"authMethod"`
		AutoJoin   []string `yaml:"autoJoin"`
		Color      string   `yaml:"color"`
		Initial    string   `yaml:"initial"`
	} `yaml:"servers"`
	WebIRC []struct {
		Host         string `yaml:"host"`
		Port         *int   `yaml:"port"`
		TLS          *bool  `yaml:"tls"`
		Gateway      string `yaml:"gateway"`
		PasswordEnv  string `yaml:"passwordEnv"`
		PasswordFile string `yaml:"passwordFile"`
	} `yaml:"webirc"`
}

func canonicalHost(host string) (string, error) {
	if ip, err := netip.ParseAddr(host); err == nil && ip.Zone() == "" {
		return ip.Unmap().String(), nil
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if strings.Contains(host, ".") && strings.Trim(host, "0123456789.") == "" {
		return "", fmt.Errorf("invalid host")
	}
	if len(host) == 0 || len(host) > 253 {
		return "", fmt.Errorf("invalid host")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid host")
		}
		for _, c := range label {
			if c < 'a' || c > 'z' {
				if c < '0' || c > '9' {
					if c != '-' {
						return "", fmt.Errorf("invalid host")
					}
				}
			}
		}
	}
	return host, nil
}

func serverEndpoint(host string, port int, tls bool) (string, error) {
	host, err := canonicalHost(host)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid endpoint")
	}
	return net.JoinHostPort(host, strconv.Itoa(port)) + "/" + strconv.FormatBool(tls), nil
}

func presetText(value string, limit int, allowSpace bool) bool {
	if value == "" || len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || (!allowSpace && unicode.IsSpace(r)) {
			return false
		}
	}
	return true
}

// LoadServerList reads bounded, strict operator configuration without including
// rejected values, file names or secret references in returned errors.
func LoadServerList(path string, passwords map[string]string, allowPrivate bool) (ServerList, error) {
	result := ServerList{Presets: []NetworkPreset{}, gateways: map[string]webIRCGateway{}}
	if path == "" {
		return result, nil
	}
	result.Configured = true
	invalid := fmt.Errorf("invalid server list configuration")
	f, err := os.Open(path)
	if err != nil {
		return ServerList{}, invalid
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(raw) > 65536 {
		return ServerList{}, invalid
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var file serverListFile
	if err = dec.Decode(&file); err != nil {
		return ServerList{}, invalid
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF || len(file.Servers) > 20 || len(file.WebIRC) > 20 {
		return ServerList{}, invalid
	}
	seen := map[string]bool{}
	for _, item := range file.Servers {
		port := 6697
		if item.Port != nil {
			port = *item.Port
		}
		secure := item.TLS == nil || *item.TLS
		host, hostErr := canonicalHost(item.Host)
		_, endpointErr := serverEndpoint(host, port, secure)
		key := net.JoinHostPort(host, strconv.Itoa(port))
		if hostErr != nil || endpointErr != nil || seen[key] || !presetText(item.Name, 64, true) || strings.TrimSpace(item.Name) != item.Name || (!secure && !allowPrivate) {
			return ServerList{}, invalid
		}
		seen[key] = true
		if item.AuthMethod == "" {
			item.AuthMethod = "none"
		}
		switch item.AuthMethod {
		case "none", "server-password", "sasl-plain", "nickserv":
		default:
			return ServerList{}, invalid
		}
		if len(item.AutoJoin) > maxSavedChannels {
			return ServerList{}, invalid
		}
		for _, channel := range item.AutoJoin {
			if !girc.IsValidChannel(channel) || !presetText(channel, 100, false) || strings.ContainsAny(channel, ",:") {
				return ServerList{}, invalid
			}
		}
		if item.Color == "" {
			item.Color = "lavender"
		}
		switch item.Color {
		case "lavender", "mint", "peach", "blue", "yellow":
		default:
			return ServerList{}, invalid
		}
		if item.Initial == "" {
			item.Initial = strings.ToUpper(string([]rune(item.Name)[0]))
		}
		if len([]rune(item.Initial)) != 1 || !presetText(item.Initial, 4, false) {
			return ServerList{}, invalid
		}
		result.Presets = append(result.Presets, NetworkPreset{Name: item.Name, Host: host, Port: port, TLS: secure, AuthMethod: item.AuthMethod, AutoJoin: append([]string{}, item.AutoJoin...), Color: item.Color, Initial: item.Initial, PasswordProvided: secure && item.AuthMethod == "server-password" && passwords[key] != ""})
	}
	seenGateways := map[string]bool{}
	for _, item := range file.WebIRC {
		port := 6697
		if item.Port != nil {
			port = *item.Port
		}
		secure := item.TLS == nil || *item.TLS
		host, hostErr := canonicalHost(item.Host)
		endpoint, err := serverEndpoint(host, port, secure)
		key := net.JoinHostPort(host, strconv.Itoa(port))
		if err != nil || hostErr != nil || (!secure && !allowPrivate) || seenGateways[key] || (item.PasswordEnv == "") == (item.PasswordFile == "") {
			return ServerList{}, invalid
		}
		seenGateways[key] = true
		if item.Gateway == "" {
			item.Gateway = "ircord"
		}
		if !presetText(item.Gateway, 32, false) || strings.Contains(item.Gateway, ":") {
			return ServerList{}, invalid
		}
		var secret string
		if item.PasswordEnv != "" {
			secret = os.Getenv(item.PasswordEnv)
		} else {
			f, err := os.Open(item.PasswordFile)
			if err != nil {
				return ServerList{}, invalid
			}
			b, readErr := io.ReadAll(io.LimitReader(f, 513))
			_ = f.Close()
			if readErr != nil || len(b) > 512 {
				return ServerList{}, invalid
			}
			secret = strings.TrimSpace(string(b))
		}
		if !presetText(secret, 512, false) || strings.Contains(secret, ":") {
			return ServerList{}, invalid
		}
		result.gateways[endpoint] = webIRCGateway{Password: secret, Gateway: item.Gateway}
	}
	return result, nil
}

func (s ServerList) gateway(host string, port int, tls bool) (webIRCGateway, bool) {
	key, err := serverEndpoint(host, port, tls)
	if err != nil {
		return webIRCGateway{}, false
	}
	gateway, ok := s.gateways[key]
	return gateway, ok
}

func (s ServerList) HasWebIRC() bool { return len(s.gateways) != 0 }
