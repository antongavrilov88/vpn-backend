package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"vpn-backend/internal/domain"
)

func TestGetAccessStatusExecuteReturnsActiveAccess(t *testing.T) {
	callLog := make([]string, 0)
	expiresAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	useCase := NewGetAccessStatusUseCase(
		&fakeUserRepository{
			callLog: &callLog,
			user:    &domain.User{ID: 42, Status: domain.UserStatusActive},
		},
		&fakeSubscriptionRepository{
			callLog: &callLog,
			activeSubscription: &domain.Subscription{
				ID:         100,
				UserID:     42,
				Status:     domain.SubscriptionStatusActive,
				ExpiresAt:  &expiresAt,
				IsLifetime: false,
			},
		},
	)

	result, err := useCase.Execute(context.Background(), GetAccessStatusInput{UserID: 42})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := &AccessStatusResult{
		AccessActive:    true,
		IsLifetime:      false,
		ExpiresAt:       &expiresAt,
		CanCreateDevice: true,
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("result = %#v, want %#v", result, want)
	}

	wantCalls := []string{"user.get_by_id", "subscription.get_active_by_user_id"}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestGetAccessStatusExecuteReturnsInviteRequiredWhenNoSubscription(t *testing.T) {
	callLog := make([]string, 0)

	useCase := NewGetAccessStatusUseCase(
		&fakeUserRepository{
			callLog: &callLog,
			user:    &domain.User{ID: 42, Status: domain.UserStatusActive},
		},
		&fakeSubscriptionRepository{
			callLog:              &callLog,
			getActiveByUserIDErr: domain.ErrSubscriptionMiss,
		},
	)
	useCase.now = func() time.Time {
		return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	}

	result, err := useCase.Execute(context.Background(), GetAccessStatusInput{UserID: 42})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := &AccessStatusResult{
		AccessActive:    false,
		CanCreateDevice: false,
		DenialReason:    domain.AccessDenialReasonInviteCodeRequired,
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("result = %#v, want %#v", result, want)
	}

	wantCalls := []string{
		"user.get_by_id",
		"subscription.get_active_by_user_id",
		"subscription.list_by_user_id",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestGetAccessStatusExecuteReturnsExpiredReason(t *testing.T) {
	expiredAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)

	useCase := NewGetAccessStatusUseCase(
		&fakeUserRepository{
			user: &domain.User{ID: 42, Status: domain.UserStatusActive},
		},
		&fakeSubscriptionRepository{
			getActiveByUserIDErr: domain.ErrSubscriptionMiss,
			listByUserID: []domain.Subscription{
				{
					ID:        1,
					UserID:    42,
					Status:    domain.SubscriptionStatusActive,
					StartsAt:  expiredAt.AddDate(0, 0, -30),
					ExpiresAt: &expiredAt,
				},
			},
		},
	)
	useCase.now = func() time.Time {
		return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	}

	result, err := useCase.Execute(context.Background(), GetAccessStatusInput{UserID: 42})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.DenialReason != domain.AccessDenialReasonExpired {
		t.Fatalf("denial reason = %q, want %q", result.DenialReason, domain.AccessDenialReasonExpired)
	}
}

func TestGetAccessStatusExecuteReturnsBlockedReasonWithoutSubscriptionLookup(t *testing.T) {
	callLog := make([]string, 0)

	useCase := NewGetAccessStatusUseCase(
		&fakeUserRepository{
			callLog: &callLog,
			user:    &domain.User{ID: 42, Status: domain.UserStatusBlocked},
		},
		&fakeSubscriptionRepository{callLog: &callLog},
	)

	result, err := useCase.Execute(context.Background(), GetAccessStatusInput{UserID: 42})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.DenialReason != domain.AccessDenialReasonUserBlocked {
		t.Fatalf("denial reason = %q, want %q", result.DenialReason, domain.AccessDenialReasonUserBlocked)
	}

	wantCalls := []string{"user.get_by_id"}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestGetAccessStatusExecuteReturnsRepositoryError(t *testing.T) {
	repositoryErr := errors.New("db down")

	useCase := NewGetAccessStatusUseCase(
		&fakeUserRepository{err: repositoryErr},
		&fakeSubscriptionRepository{},
	)

	result, err := useCase.Execute(context.Background(), GetAccessStatusInput{UserID: 42})
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("Execute() error = %v, want %v", err, repositoryErr)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}
}

type fakeSubscriptionRepository struct {
	callLog              *[]string
	activeSubscription   *domain.Subscription
	getActiveByUserIDErr error
	listByUserID         []domain.Subscription
	listByUserIDErr      error
	createResult         *domain.Subscription
	createErr            error
	updateResult         *domain.Subscription
	updateErr            error
}

func (f *fakeSubscriptionRepository) GetByID(context.Context, int64) (*domain.Subscription, error) {
	return nil, nil
}

func (f *fakeSubscriptionRepository) ListByUserID(context.Context, int64) ([]domain.Subscription, error) {
	if f.callLog != nil {
		*f.callLog = append(*f.callLog, "subscription.list_by_user_id")
	}
	return f.listByUserID, f.listByUserIDErr
}

func (f *fakeSubscriptionRepository) GetActiveByUserID(context.Context, int64) (*domain.Subscription, error) {
	if f.callLog != nil {
		*f.callLog = append(*f.callLog, "subscription.get_active_by_user_id")
	}
	return f.activeSubscription, f.getActiveByUserIDErr
}

func (f *fakeSubscriptionRepository) Create(context.Context, domain.Subscription) (*domain.Subscription, error) {
	return f.createResult, f.createErr
}

func (f *fakeSubscriptionRepository) Update(context.Context, domain.Subscription) (*domain.Subscription, error) {
	return f.updateResult, f.updateErr
}
