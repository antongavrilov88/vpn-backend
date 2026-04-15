package proxy

import "testing"

func TestPeerRoute(t *testing.T) {
	if got := peerRoute("10.68.0.2"); got != "10.68.0.2/32" {
		t.Fatalf("peerRoute() = %q, want %q", got, "10.68.0.2/32")
	}
}
