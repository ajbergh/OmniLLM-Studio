package gitrepo

import (
	"net"
	"testing"
)

func TestRemoteEgressBlocksPrivateAndMetadataAddresses(t *testing.T) {
	blocked := []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "192.168.1.10", "::1", "fc00::1", "fe80::1"}
	for _, raw := range blocked {
		if !isBlockedRemoteIP(net.ParseIP(raw)) {
			t.Fatalf("%s was not blocked", raw)
		}
	}
	if isBlockedRemoteIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public test address was unexpectedly blocked")
	}
}
