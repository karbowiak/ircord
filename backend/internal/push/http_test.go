package push

import (
	"net/netip"
	"testing"
)

func TestPushDestinationsStayPublicAfterPreviewRemoval(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "198.18.0.1", "64:ff9b::7f00:1", "2002:7f00:1::"} {
		if publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("accepted private/reserved push destination %s", address)
		}
	}
	for _, address := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("rejected public push destination %s", address)
		}
	}
}
