package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"vpn-backend/internal/domain"
)

func TestListUserDevicesExecuteHappyPath(t *testing.T) {
	callLog := make([]string, 0)
	createdAt := time.Date(2026, 4, 2, 10, 11, 12, 0, time.UTC)
	revokedAt := time.Date(2026, 4, 3, 10, 11, 12, 0, time.UTC)

	useCase := NewListUserDevicesUseCase(
		&fakeUserRepository{
			callLog: &callLog,
			user: &domain.User{
				ID:     42,
				Status: domain.UserStatusActive,
			},
		},
		&fakeDeviceRepository{
			callLog: &callLog,
			listByUserID: []domain.Device{
				{
					ID:                  100,
					UserID:              42,
					Name:                "Dad Phone",
					PublicKey:           "public-key",
					EncryptedPrivateKey: "encrypted-private-key",
					AssignedIP:          "10.67.0.2",
					Status:              domain.DeviceStatusActive,
					CreatedAt:           createdAt,
				},
				{
					ID:                  101,
					UserID:              42,
					Name:                "Old Tablet",
					PublicKey:           "public-key-2",
					EncryptedPrivateKey: "encrypted-private-key-2",
					AssignedIP:          "10.67.0.3",
					Status:              domain.DeviceStatusRevoked,
					CreatedAt:           createdAt.Add(time.Hour),
					RevokedAt:           &revokedAt,
				},
			},
		},
	)

	result, err := useCase.Execute(context.Background(), ListUserDevicesInput{
		CallerUserID: 42,
		UserID:       42,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}

	want := []ListUserDevicesItem{
		{
			ID:         100,
			Name:       "Dad Phone",
			AssignedIP: "10.67.0.2",
			Status:     domain.DeviceStatusActive,
			CreatedAt:  createdAt,
		},
		{
			ID:         101,
			Name:       "Old Tablet",
			AssignedIP: "10.67.0.3",
			Status:     domain.DeviceStatusRevoked,
			CreatedAt:  createdAt.Add(time.Hour),
			RevokedAt:  &revokedAt,
		},
	}
	if !reflect.DeepEqual(result.Devices, want) {
		t.Fatalf("Devices = %#v, want %#v", result.Devices, want)
	}

	wantCalls := []string{
		"user.get_by_id",
		"device.list_by_user_id",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestListUserDevicesExecuteReturnsNotFoundWhenUserMissing(t *testing.T) {
	useCase := NewListUserDevicesUseCase(
		&fakeUserRepository{err: domain.ErrNotFound},
		&fakeDeviceRepository{},
	)

	result, err := useCase.Execute(context.Background(), ListUserDevicesInput{
		CallerUserID: 42,
		UserID:       42,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrNotFound)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}
}

func TestListUserDevicesExecuteReturnsNotFoundForForeignUser(t *testing.T) {
	callLog := make([]string, 0)

	useCase := NewListUserDevicesUseCase(
		&fakeUserRepository{callLog: &callLog},
		&fakeDeviceRepository{callLog: &callLog},
	)

	result, err := useCase.Execute(context.Background(), ListUserDevicesInput{
		CallerUserID: 42,
		UserID:       99,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrNotFound)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}

	if len(callLog) != 0 {
		t.Fatalf("call log = %#v, want empty", callLog)
	}
}
