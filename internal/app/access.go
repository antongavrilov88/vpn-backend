package app

import (
	"context"

	"vpn-backend/internal/domain"
)

func loadAccessibleUser(ctx context.Context, userRepository domain.UserRepository, userID int64) (*domain.User, error) {
	user, err := userRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user.Status == domain.UserStatusBlocked {
		return nil, domain.ErrUserBlocked
	}

	if user.Status == domain.UserStatusDeleted {
		return nil, domain.ErrUserDeleted
	}

	return user, nil
}
