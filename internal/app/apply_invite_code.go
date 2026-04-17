package app

import (
	"context"
	"strings"
	"time"

	"vpn-backend/internal/domain"
)

type inviteCodeGrantor interface {
	ApplyInviteCode(ctx context.Context, userID int64, code string, now time.Time) (*domain.Subscription, error)
}

type ApplyInviteCodeUseCase struct {
	userRepository domain.UserRepository
	grantor        inviteCodeGrantor
	now            func() time.Time
}

type ApplyInviteCodeInput struct {
	UserID int64
	Code   string
}

func NewApplyInviteCodeUseCase(
	userRepository domain.UserRepository,
	grantor inviteCodeGrantor,
) *ApplyInviteCodeUseCase {
	return &ApplyInviteCodeUseCase{
		userRepository: userRepository,
		grantor:        grantor,
		now:            time.Now,
	}
}

func (uc *ApplyInviteCodeUseCase) Execute(ctx context.Context, input ApplyInviteCodeInput) (*AccessStatusResult, error) {
	if _, err := loadAccessibleUser(ctx, uc.userRepository, input.UserID); err != nil {
		return nil, err
	}

	subscription, err := uc.grantor.ApplyInviteCode(ctx, input.UserID, strings.TrimSpace(input.Code), uc.now().UTC())
	if err != nil {
		return nil, err
	}

	return activeAccessStatus(subscription), nil
}
