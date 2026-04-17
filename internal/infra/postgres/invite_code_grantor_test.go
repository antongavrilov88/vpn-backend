package postgres

import (
	"testing"

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
