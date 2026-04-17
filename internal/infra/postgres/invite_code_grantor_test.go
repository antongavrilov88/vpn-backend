package postgres

import (
	"testing"
	"time"

	"vpn-backend/internal/domain"
)

func TestShouldSkipInviteCodeConsumption(t *testing.T) {
	tests := []struct {
		name               string
		activeSubscription *domain.Subscription
		want               bool
	}{
		{
			name: "nil subscription",
			want: false,
		},
		{
			name: "active non-lifetime subscription",
			activeSubscription: &domain.Subscription{
				IsLifetime: false,
			},
			want: false,
		},
		{
			name: "active lifetime subscription",
			activeSubscription: &domain.Subscription{
				IsLifetime: true,
			},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := shouldSkipInviteCodeConsumption(test.activeSubscription)
			if got != test.want {
				t.Fatalf("shouldSkipInviteCodeConsumption() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestImmediateInviteActivationStartBackdatesTimestamp(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	got := immediateInviteActivationStart(now)
	want := now.Add(-immediateInviteActivationSkew)

	if !got.Equal(want) {
		t.Fatalf("immediateInviteActivationStart() = %v, want %v", got, want)
	}
}
