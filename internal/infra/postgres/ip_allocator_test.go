package postgres

import (
	"net/netip"
	"testing"
)

func TestAllocatorNetworkConfigUsesConfiguredTunnelAddress(t *testing.T) {
	subnet, reservedIP, err := allocatorNetworkConfig("10.68.0.1/24")
	if err != nil {
		t.Fatalf("allocatorNetworkConfig() error = %v", err)
	}

	if got := subnet.String(); got != "10.68.0.0/24" {
		t.Fatalf("subnet = %q, want %q", got, "10.68.0.0/24")
	}

	if got := reservedIP.String(); got != "10.68.0.1" {
		t.Fatalf("reservedIP = %q, want %q", got, "10.68.0.1")
	}
}

func TestAllocatorNetworkConfigUsesDefaultTunnelAddress(t *testing.T) {
	subnet, reservedIP, err := allocatorNetworkConfig("")
	if err != nil {
		t.Fatalf("allocatorNetworkConfig() error = %v", err)
	}

	if got := subnet.String(); got != "10.67.0.0/24" {
		t.Fatalf("subnet = %q, want %q", got, "10.67.0.0/24")
	}

	if got := reservedIP.String(); got != "10.67.0.1" {
		t.Fatalf("reservedIP = %q, want %q", got, "10.67.0.1")
	}
}

func TestBuildUsedIPSetAcceptsHostAddressAndCIDRText(t *testing.T) {
	usedIPs, err := buildUsedIPSet([]string{"10.68.0.2", "10.68.0.3/32"})
	if err != nil {
		t.Fatalf("buildUsedIPSet() error = %v", err)
	}

	if _, ok := usedIPs[mustAddr("10.68.0.2")]; !ok {
		t.Fatal("used IP set is missing 10.68.0.2")
	}

	if _, ok := usedIPs[mustAddr("10.68.0.3")]; !ok {
		t.Fatal("used IP set is missing 10.68.0.3")
	}
}

func mustAddr(raw string) netip.Addr {
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		panic(err)
	}

	return addr
}
