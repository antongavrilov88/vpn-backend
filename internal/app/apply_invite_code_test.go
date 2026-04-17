package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"vpn-backend/internal/domain"
)

func TestApplyInviteCodeExecuteGrantsLifetimeAccess(t *testing.T) {
	expiresAt := (*time.Time)(nil)
	grantor := &fakeInviteCodeGrantor{
		subscription: &domain.Subscription{
			ID:         100,
			UserID:     42,
			PlanCode:   "closed_beta_invite",
			Status:     domain.SubscriptionStatusActive,
			StartsAt:   time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
			ExpiresAt:  expiresAt,
			IsLifetime: true,
			Source:     domain.SubscriptionSourceTelegram,
		},
	}

	useCase := NewApplyInviteCodeUseCase(
		&fakeUserRepository{
			user: &domain.User{ID: 42, Status: domain.UserStatusActive},
		},
		grantor,
	)
	useCase.now = func() time.Time {
		return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	}

	result, err := useCase.Execute(context.Background(), ApplyInviteCodeInput{
		UserID: 42,
		Code:   "  BETA-ANTON  ",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if grantor.userID != 42 {
		t.Fatalf("grantor user id = %d, want %d", grantor.userID, 42)
	}

	if grantor.code != "BETA-ANTON" {
		t.Fatalf("grantor code = %q, want %q", grantor.code, "BETA-ANTON")
	}

	if result == nil || !result.AccessActive || !result.IsLifetime || !result.CanCreateDevice {
		t.Fatalf("result = %#v, want active lifetime access", result)
	}

	if result.ExpiresAt != nil {
		t.Fatalf("expires_at = %#v, want nil", result.ExpiresAt)
	}
}

func TestApplyInviteCodeExecuteReturnsPromoError(t *testing.T) {
	grantor := &fakeInviteCodeGrantor{err: domain.ErrPromoCodeUsed}

	useCase := NewApplyInviteCodeUseCase(
		&fakeUserRepository{
			user: &domain.User{ID: 42, Status: domain.UserStatusActive},
		},
		grantor,
	)

	result, err := useCase.Execute(context.Background(), ApplyInviteCodeInput{
		UserID: 42,
		Code:   "BETA-ANTON",
	})
	if !errors.Is(err, domain.ErrPromoCodeUsed) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrPromoCodeUsed)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}
}

func TestApplyInviteCodeExecuteReturnsBlockedUserErrorBeforeGrantor(t *testing.T) {
	grantor := &fakeInviteCodeGrantor{}

	useCase := NewApplyInviteCodeUseCase(
		&fakeUserRepository{
			user: &domain.User{ID: 42, Status: domain.UserStatusBlocked},
		},
		grantor,
	)

	result, err := useCase.Execute(context.Background(), ApplyInviteCodeInput{
		UserID: 42,
		Code:   "BETA-ANTON",
	})
	if !errors.Is(err, domain.ErrUserBlocked) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrUserBlocked)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}

	if grantor.calls != 0 {
		t.Fatalf("grantor calls = %d, want %d", grantor.calls, 0)
	}
}

type fakeInviteCodeGrantor struct {
	calls        int
	userID       int64
	code         string
	now          time.Time
	subscription *domain.Subscription
	err          error
}

func (f *fakeInviteCodeGrantor) ApplyInviteCode(_ context.Context, userID int64, code string, now time.Time) (*domain.Subscription, error) {
	f.calls++
	f.userID = userID
	f.code = code
	f.now = now
	return f.subscription, f.err
}
