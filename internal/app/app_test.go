package app

import (
	"reflect"
	"testing"
)

func TestEffectiveClientAllowedIPsDefaultsToIPv4FullTunnel(t *testing.T) {
	got := effectiveClientAllowedIPs(nil)
	want := []string{"0.0.0.0/0"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("effectiveClientAllowedIPs(nil) = %#v, want %#v", got, want)
	}
}

func TestEffectiveClientAllowedIPsPreservesConfiguredOverride(t *testing.T) {
	configured := []string{"0.0.0.0/0", "::/0"}

	got := effectiveClientAllowedIPs(configured)
	if !reflect.DeepEqual(got, configured) {
		t.Fatalf("effectiveClientAllowedIPs(%#v) = %#v, want %#v", configured, got, configured)
	}

	got[0] = "10.0.0.0/8"
	if configured[0] != "0.0.0.0/0" {
		t.Fatalf("configured slice was mutated: %#v", configured)
	}
}
