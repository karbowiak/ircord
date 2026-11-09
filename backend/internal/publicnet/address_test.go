package publicnet

import (
	"net/netip"
	"testing"
)

func TestIsPublic(t *testing.T) {
	for _, raw := range []string{
		"0.0.0.0", "0.1.2.3", "10.0.0.1", "127.0.0.1", "169.254.169.254",
		"172.16.0.1", "192.168.0.1", "100.64.0.1", "100.127.255.255",
		"192.0.0.1", "192.0.2.1", "198.18.0.1", "198.19.255.255",
		"198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1", "255.255.255.255",
		"::", "::1", "::ffff:127.0.0.1", "::ffff:192.0.2.1", "fd00::1", "fe80::1",
		"ff02::1", "64:ff9b::7f00:1", "2002:7f00:1::",
	} {
		t.Run(raw, func(t *testing.T) {
			if IsPublic(netip.MustParseAddr(raw)) {
				t.Fatal("accepted private or reserved address")
			}
		})
	}
	for _, raw := range []string{"1.1.1.1", "100.63.255.255", "100.128.0.0", "::ffff:1.1.1.1", "2606:4700:4700::1111"} {
		t.Run(raw, func(t *testing.T) {
			if !IsPublic(netip.MustParseAddr(raw)) {
				t.Fatal("rejected public address")
			}
		})
	}
	if IsPublic(netip.Addr{}) {
		t.Fatal("accepted invalid address")
	}
}
