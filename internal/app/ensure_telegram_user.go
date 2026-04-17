package app

import (
	"context"
	"errors"
	"strings"

	"vpn-backend/internal/domain"
)

type EnsureTelegramUserUseCase struct {
	userRepository domain.UserRepository
}

type EnsureTelegramUserInput struct {
	TelegramUserID int64
	Username       string
}

type EnsureTelegramUserResult struct {
	User *domain.User
}

func NewEnsureTelegramUserUseCase(userRepository domain.UserRepository) *EnsureTelegramUserUseCase {
	return &EnsureTelegramUserUseCase{
		userRepository: userRepository,
	}
}

func (uc *EnsureTelegramUserUseCase) Execute(ctx context.Context, input EnsureTelegramUserInput) (*EnsureTelegramUserResult, error) {
	username := strings.TrimSpace(input.Username)

	user, err := uc.userRepository.GetByTelegramID(ctx, input.TelegramUserID)
	switch {
	case err == nil:
		user, err = uc.maybeUpdateUsername(ctx, user, username)
		if err != nil {
			return nil, err
		}

		return &EnsureTelegramUserResult{User: user}, nil
	case !errors.Is(err, domain.ErrNotFound):
		return nil, err
	}

	telegramUserID := input.TelegramUserID
	user, err = uc.userRepository.Create(ctx, domain.User{
		TelegramID: &telegramUserID,
		Username:   username,
		Status:     domain.UserStatusActive,
	})
	if err == nil {
		return &EnsureTelegramUserResult{User: user}, nil
	}

	if !errors.Is(err, domain.ErrConflict) {
		return nil, err
	}

	user, err = uc.userRepository.GetByTelegramID(ctx, input.TelegramUserID)
	if err != nil {
		return nil, err
	}

	user, err = uc.maybeUpdateUsername(ctx, user, username)
	if err != nil {
		return nil, err
	}

	return &EnsureTelegramUserResult{User: user}, nil
}

func (uc *EnsureTelegramUserUseCase) maybeUpdateUsername(ctx context.Context, user *domain.User, username string) (*domain.User, error) {
	if user == nil || username == "" || user.Username == username {
		return user, nil
	}

	updatedUser := *user
	updatedUser.Username = username

	return uc.userRepository.Update(ctx, updatedUser)
}
