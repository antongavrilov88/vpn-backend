package app

import (
	"context"
	"errors"
	"testing"

	"vpn-backend/internal/domain"
)

func TestEnsureTelegramUserExecuteReturnsExistingUser(t *testing.T) {
	telegramUserID := int64(777)
	repository := &ensureTelegramUserRepository{
		getByTelegramIDUser: &domain.User{
			ID:         42,
			TelegramID: &telegramUserID,
			Username:   "existing-user",
			Status:     domain.UserStatusActive,
		},
	}

	useCase := NewEnsureTelegramUserUseCase(repository)

	result, err := useCase.Execute(context.Background(), EnsureTelegramUserInput{
		TelegramUserID: telegramUserID,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil || result.User == nil {
		t.Fatal("Execute() result/user = nil, want non-nil")
	}

	if result.User.ID != 42 {
		t.Fatalf("User.ID = %d, want %d", result.User.ID, 42)
	}

	if repository.createCalls != 0 {
		t.Fatalf("create calls = %d, want %d", repository.createCalls, 0)
	}
}

func TestEnsureTelegramUserExecuteCreatesMissingUser(t *testing.T) {
	telegramUserID := int64(777)
	repository := &ensureTelegramUserRepository{
		getByTelegramIDErr: domain.ErrNotFound,
		createResult: &domain.User{
			ID:         42,
			TelegramID: &telegramUserID,
			Username:   "new-user",
			Status:     domain.UserStatusActive,
		},
	}

	useCase := NewEnsureTelegramUserUseCase(repository)

	result, err := useCase.Execute(context.Background(), EnsureTelegramUserInput{
		TelegramUserID: telegramUserID,
		Username:       "new-user",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil || result.User == nil {
		t.Fatal("Execute() result/user = nil, want non-nil")
	}

	if repository.createCalls != 1 {
		t.Fatalf("create calls = %d, want %d", repository.createCalls, 1)
	}

	if repository.createdUser == nil {
		t.Fatal("created user = nil, want non-nil")
	}

	if repository.createdUser.TelegramID == nil || *repository.createdUser.TelegramID != telegramUserID {
		t.Fatalf("created telegram id = %#v, want %d", repository.createdUser.TelegramID, telegramUserID)
	}

	if repository.createdUser.Status != domain.UserStatusActive {
		t.Fatalf("created status = %q, want %q", repository.createdUser.Status, domain.UserStatusActive)
	}
}

func TestEnsureTelegramUserExecuteUpdatesUsernameWhenChanged(t *testing.T) {
	telegramUserID := int64(777)
	repository := &ensureTelegramUserRepository{
		getByTelegramIDUser: &domain.User{
			ID:         42,
			TelegramID: &telegramUserID,
			Username:   "old-user",
			Status:     domain.UserStatusActive,
		},
		updateResult: &domain.User{
			ID:         42,
			TelegramID: &telegramUserID,
			Username:   "new-user",
			Status:     domain.UserStatusActive,
		},
	}

	useCase := NewEnsureTelegramUserUseCase(repository)

	result, err := useCase.Execute(context.Background(), EnsureTelegramUserInput{
		TelegramUserID: telegramUserID,
		Username:       "new-user",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil || result.User == nil {
		t.Fatal("Execute() result/user = nil, want non-nil")
	}

	if repository.updateCalls != 1 {
		t.Fatalf("update calls = %d, want %d", repository.updateCalls, 1)
	}

	if repository.updatedUser == nil || repository.updatedUser.Username != "new-user" {
		t.Fatalf("updated user = %#v, want username new-user", repository.updatedUser)
	}
}

func TestEnsureTelegramUserExecuteReturnsExistingUserAfterCreateConflict(t *testing.T) {
	telegramUserID := int64(777)
	repository := &ensureTelegramUserRepository{
		getByTelegramIDSequence: []ensureTelegramLookup{
			{err: domain.ErrNotFound},
			{
				user: &domain.User{
					ID:         42,
					TelegramID: &telegramUserID,
					Username:   "new-user",
					Status:     domain.UserStatusActive,
				},
			},
		},
		createErr: domain.ErrConflict,
	}

	useCase := NewEnsureTelegramUserUseCase(repository)

	result, err := useCase.Execute(context.Background(), EnsureTelegramUserInput{
		TelegramUserID: telegramUserID,
		Username:       "new-user",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil || result.User == nil || result.User.ID != 42 {
		t.Fatalf("result = %#v, want user id 42", result)
	}

	if repository.getByTelegramIDCalls != 2 {
		t.Fatalf("get by telegram calls = %d, want %d", repository.getByTelegramIDCalls, 2)
	}
}

func TestEnsureTelegramUserExecuteReturnsRepositoryError(t *testing.T) {
	repository := &ensureTelegramUserRepository{
		getByTelegramIDErr: errors.New("db down"),
	}

	useCase := NewEnsureTelegramUserUseCase(repository)

	result, err := useCase.Execute(context.Background(), EnsureTelegramUserInput{
		TelegramUserID: 777,
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}
}

type ensureTelegramLookup struct {
	user *domain.User
	err  error
}

type ensureTelegramUserRepository struct {
	getByTelegramIDUser     *domain.User
	getByTelegramIDErr      error
	getByTelegramIDCalls    int
	getByTelegramIDSequence []ensureTelegramLookup
	createCalls             int
	createResult            *domain.User
	createErr               error
	createdUser             *domain.User
	updateCalls             int
	updateResult            *domain.User
	updateErr               error
	updatedUser             *domain.User
}

func (r *ensureTelegramUserRepository) GetByID(context.Context, int64) (*domain.User, error) {
	return nil, nil
}

func (r *ensureTelegramUserRepository) GetByTelegramID(context.Context, int64) (*domain.User, error) {
	r.getByTelegramIDCalls++
	if len(r.getByTelegramIDSequence) > 0 {
		next := r.getByTelegramIDSequence[0]
		r.getByTelegramIDSequence = r.getByTelegramIDSequence[1:]
		return next.user, next.err
	}

	return r.getByTelegramIDUser, r.getByTelegramIDErr
}

func (r *ensureTelegramUserRepository) Create(_ context.Context, user domain.User) (*domain.User, error) {
	r.createCalls++
	userCopy := user
	r.createdUser = &userCopy
	if r.createErr != nil {
		return nil, r.createErr
	}

	return r.createResult, nil
}

func (r *ensureTelegramUserRepository) Update(_ context.Context, user domain.User) (*domain.User, error) {
	r.updateCalls++
	userCopy := user
	r.updatedUser = &userCopy
	if r.updateErr != nil {
		return nil, r.updateErr
	}

	return r.updateResult, nil
}
