package app

import (
	"context"
	"errors"
	"time"

	"vpn-backend/internal/domain"
)

type AccessStatusResult struct {
	AccessActive    bool
	IsLifetime      bool
	ExpiresAt       *time.Time
	CanCreateDevice bool
	DenialReason    domain.AccessDenialReason
}

type GetAccessStatusUseCase struct {
	userRepository         domain.UserRepository
	subscriptionRepository domain.SubscriptionRepository
	now                    func() time.Time
}

type GetAccessStatusInput struct {
	UserID int64
}

func NewGetAccessStatusUseCase(
	userRepository domain.UserRepository,
	subscriptionRepository domain.SubscriptionRepository,
) *GetAccessStatusUseCase {
	return &GetAccessStatusUseCase{
		userRepository:         userRepository,
		subscriptionRepository: subscriptionRepository,
		now:                    time.Now,
	}
}

func (uc *GetAccessStatusUseCase) Execute(ctx context.Context, input GetAccessStatusInput) (*AccessStatusResult, error) {
	user, err := uc.userRepository.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	switch user.Status {
	case domain.UserStatusBlocked:
		return inactiveAccessStatus(domain.AccessDenialReasonUserBlocked), nil
	case domain.UserStatusDeleted:
		return inactiveAccessStatus(domain.AccessDenialReasonUserDeleted), nil
	}

	if uc.subscriptionRepository == nil {
		return inactiveAccessStatus(domain.AccessDenialReasonInviteCodeRequired), nil
	}

	subscription, err := uc.subscriptionRepository.GetActiveByUserID(ctx, input.UserID)
	if err == nil {
		return activeAccessStatus(subscription), nil
	}

	if !errors.Is(err, domain.ErrSubscriptionMiss) {
		return nil, err
	}

	subscriptions, err := uc.subscriptionRepository.ListByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	return inactiveAccessStatus(deriveAccessDenialReason(subscriptions, uc.now().UTC())), nil
}

func activeAccessStatus(subscription *domain.Subscription) *AccessStatusResult {
	if subscription == nil {
		return inactiveAccessStatus(domain.AccessDenialReasonInviteCodeRequired)
	}

	return &AccessStatusResult{
		AccessActive:    true,
		IsLifetime:      subscription.IsLifetime,
		ExpiresAt:       subscription.ExpiresAt,
		CanCreateDevice: true,
	}
}

func inactiveAccessStatus(reason domain.AccessDenialReason) *AccessStatusResult {
	return &AccessStatusResult{
		AccessActive:    false,
		CanCreateDevice: false,
		DenialReason:    reason,
	}
}

func deriveAccessDenialReason(subscriptions []domain.Subscription, now time.Time) domain.AccessDenialReason {
	if len(subscriptions) == 0 {
		return domain.AccessDenialReasonInviteCodeRequired
	}

	subscription := subscriptions[0]
	switch subscription.Status {
	case domain.SubscriptionStatusPending:
		return domain.AccessDenialReasonPending
	case domain.SubscriptionStatusCanceled:
		return domain.AccessDenialReasonCanceled
	case domain.SubscriptionStatusExpired:
		return domain.AccessDenialReasonExpired
	case domain.SubscriptionStatusActive:
		if subscription.StartsAt.After(now) {
			return domain.AccessDenialReasonPending
		}
		if subscription.ExpiresAt != nil && subscription.ExpiresAt.Before(now) {
			return domain.AccessDenialReasonExpired
		}
	}

	if subscription.ExpiresAt != nil && subscription.ExpiresAt.Before(now) {
		return domain.AccessDenialReasonExpired
	}

	return domain.AccessDenialReasonInviteCodeRequired
}
