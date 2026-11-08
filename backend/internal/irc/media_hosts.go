package irc

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// maxTrustedMediaHosts bounds a channel's extra trusted media hosts.
const maxTrustedMediaHosts = 20

// normalMediaHosts checks a channel's extra trusted media hosts: exact host names whose direct
// media links members' clients embed under the "trusted" preview setting, like the built-in
// well-known hosts. Entries are trimmed, lower-cased and de-duplicated; schemes, ports, paths,
// wildcards, IP addresses and names outside the public DNS are refused so a typo cannot widen
// trust.
func normalMediaHosts(entries []string) ([]string, error) {
	hosts := []string{}
	for _, entry := range entries {
		host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(entry)), ".")
		if host == "" {
			continue
		}
		if err := checkMediaHost(host); err != nil {
			return nil, fmt.Errorf("trusted media host %q %s", strings.TrimSpace(entry), err)
		}
		if !slices.Contains(hosts, host) {
			hosts = append(hosts, host)
		}
	}
	if len(hosts) > maxTrustedMediaHosts {
		return nil, fmt.Errorf("a channel can trust at most %d media hosts", maxTrustedMediaHosts)
	}
	return hosts, nil
}

func checkMediaHost(host string) error {
	if strings.Contains(host, "*") {
		return fmt.Errorf("must be an exact host name; wildcards are not supported")
	}
	if strings.ContainsAny(host, ":/?#@[]\\") {
		return fmt.Errorf("must be a bare host name such as images.example.com, without scheme, port or path")
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return fmt.Errorf("must be a host name, not an IP address")
	}
	if len(host) > 253 {
		return fmt.Errorf("is too long")
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return fmt.Errorf("must be a full host name such as images.example.com")
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("is not a valid host name")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return fmt.Errorf("is not a valid host name")
			}
		}
	}
	// Browsers read a host whose last label is a number (127.1, 0x7f.1) as an IPv4 address.
	if numericLabel(labels[len(labels)-1]) {
		return fmt.Errorf("must be a host name, not an IP address")
	}
	// Only names under a public suffix (com, co.uk, github.io, …) are public; anything else
	// (.local, .corp, .test, …) is a private or local name, and a bare suffix names no one host.
	suffix, icann := publicsuffix.PublicSuffix(host)
	if !icann && !strings.Contains(suffix, ".") || labels[len(labels)-1] == "arpa" {
		return fmt.Errorf("names a private or local host")
	}
	if host == suffix {
		return fmt.Errorf("is a public suffix, not a host name")
	}
	return nil
}

// numericLabel reports a label that URL parsing treats as a number: decimal digits, or 0x
// followed by hex digits.
func numericLabel(label string) bool {
	digits := "0123456789"
	if rest, ok := strings.CutPrefix(label, "0x"); ok {
		label, digits = rest, "0123456789abcdef"
		if label == "" {
			return true
		}
	}
	return strings.Trim(label, digits) == ""
}
