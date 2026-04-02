package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"vpn-backend/internal/domain"
)

func TestRevokeDeviceExecuteHappyPath(t *testing.T) {
	callLog := make([]string, 0)
	revokedAt := time.Date(2026, 4, 2, 10, 11, 12, 0, time.UTC)

	userRepository := &fakeUserRepository{
		callLog: &callLog,
		user: &domain.User{
			ID:     42,
			Status: domain.UserStatusActive,
		},
	}
	deviceRepository := &fakeDeviceRepository{
		callLog: &callLog,
		getByIDDevice: &domain.Device{
			ID:         100,
			UserID:     42,
			Name:       "Dad Phone",
			PublicKey:  "public-key",
			AssignedIP: "10.67.0.2",
			Status:     domain.DeviceStatusActive,
		},
		updateResult: &domain.Device{
			ID:         100,
			UserID:     42,
			Name:       "Dad Phone",
			PublicKey:  "public-key",
			AssignedIP: "10.67.0.2",
			Status:     domain.DeviceStatusRevoked,
			RevokedAt:  &revokedAt,
		},
	}
	transport := &fakeVPNTransport{callLog: &callLog}

	useCase := NewRevokeDeviceUseCase(
		userRepository,
		deviceRepository,
		transport,
	)
	useCase.now = func() time.Time { return revokedAt }

	result, err := useCase.Execute(context.Background(), RevokeDeviceInput{
		UserID:   42,
		DeviceID: 100,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil || result.Device == nil {
		t.Fatal("Execute() result/device = nil, want non-nil")
	}

	if result.Device.Status != domain.DeviceStatusRevoked {
		t.Fatalf("Device.Status = %q, want %q", result.Device.Status, domain.DeviceStatusRevoked)
	}

	if result.Device.RevokedAt == nil || !result.Device.RevokedAt.Equal(revokedAt) {
		t.Fatalf("Device.RevokedAt = %v, want %v", result.Device.RevokedAt, revokedAt)
	}

	if got := transport.removePeerInput; got.PublicKey != "public-key" || got.AssignedIP != "10.67.0.2" {
		t.Fatalf("RemovePeer input = %#v, want public-key/10.67.0.2", got)
	}

	if deviceRepository.updatedDevice == nil {
		t.Fatal("updated device = nil, want non-nil")
	}

	if deviceRepository.updatedDevice.Status != domain.DeviceStatusRevoked {
		t.Fatalf("updated device status = %q, want %q", deviceRepository.updatedDevice.Status, domain.DeviceStatusRevoked)
	}

	if deviceRepository.updatedDevice.RevokedAt == nil || !deviceRepository.updatedDevice.RevokedAt.Equal(revokedAt) {
		t.Fatalf("updated device revoked_at = %v, want %v", deviceRepository.updatedDevice.RevokedAt, revokedAt)
	}

	wantCalls := []string{
		"device.get_by_id",
		"user.get_by_id",
		"transport.remove_peer",
		"device.update",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestRevokeDeviceExecuteReturnsNotFoundWhenDeviceMissing(t *testing.T) {
	useCase := NewRevokeDeviceUseCase(
		&fakeUserRepository{},
		&fakeDeviceRepository{getByIDErr: domain.ErrNotFound},
		&fakeVPNTransport{},
	)

	result, err := useCase.Execute(context.Background(), RevokeDeviceInput{
		UserID:   42,
		DeviceID: 100,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrNotFound)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}
}

func TestRevokeDeviceExecuteReturnsDeviceRevokedWhenAlreadyRevoked(t *testing.T) {
	callLog := make([]string, 0)
	revokedAt := time.Date(2026, 4, 1, 10, 11, 12, 0, time.UTC)

	useCase := NewRevokeDeviceUseCase(
		&fakeUserRepository{
			callLog: &callLog,
			user: &domain.User{
				ID:     42,
				Status: domain.UserStatusActive,
			},
		},
		&fakeDeviceRepository{
			callLog: &callLog,
			getByIDDevice: &domain.Device{
				ID:         100,
				UserID:     42,
				PublicKey:  "public-key",
				AssignedIP: "10.67.0.2",
				Status:     domain.DeviceStatusRevoked,
				RevokedAt:  &revokedAt,
			},
		},
		&fakeVPNTransport{callLog: &callLog},
	)

	result, err := useCase.Execute(context.Background(), RevokeDeviceInput{
		UserID:   42,
		DeviceID: 100,
	})
	if !errors.Is(err, domain.ErrDeviceRevoked) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrDeviceRevoked)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}

	wantCalls := []string{
		"device.get_by_id",
		"user.get_by_id",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestRevokeDeviceExecuteReturnsTransportErrorWithoutPersisting(t *testing.T) {
	callLog := make([]string, 0)
	removeErr := errors.New("remove peer failed")

	deviceRepository := &fakeDeviceRepository{
		callLog: &callLog,
		getByIDDevice: &domain.Device{
			ID:         100,
			UserID:     42,
			PublicKey:  "public-key",
			AssignedIP: "10.67.0.2",
			Status:     domain.DeviceStatusActive,
		},
	}
	transport := &fakeVPNTransport{
		callLog:       &callLog,
		removePeerErr: removeErr,
	}

	useCase := NewRevokeDeviceUseCase(
		&fakeUserRepository{
			callLog: &callLog,
			user: &domain.User{
				ID:     42,
				Status: domain.UserStatusActive,
			},
		},
		deviceRepository,
		transport,
	)

	result, err := useCase.Execute(context.Background(), RevokeDeviceInput{
		UserID:   42,
		DeviceID: 100,
	})
	if !errors.Is(err, removeErr) {
		t.Fatalf("Execute() error = %v, want %v", err, removeErr)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}

	if deviceRepository.updateCalls != 0 {
		t.Fatalf("device update calls = %d, want %d", deviceRepository.updateCalls, 0)
	}

	wantCalls := []string{
		"device.get_by_id",
		"user.get_by_id",
		"transport.remove_peer",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestRevokeDeviceExecuteReturnsNotFoundForForeignDevice(t *testing.T) {
	callLog := make([]string, 0)

	useCase := NewRevokeDeviceUseCase(
		&fakeUserRepository{
			callLog: &callLog,
			user: &domain.User{
				ID:     42,
				Status: domain.UserStatusActive,
			},
		},
		&fakeDeviceRepository{
			callLog: &callLog,
			getByIDDevice: &domain.Device{
				ID:         100,
				UserID:     99,
				PublicKey:  "public-key",
				AssignedIP: "10.67.0.2",
				Status:     domain.DeviceStatusActive,
			},
		},
		&fakeVPNTransport{callLog: &callLog},
	)

	result, err := useCase.Execute(context.Background(), RevokeDeviceInput{
		UserID:   42,
		DeviceID: 100,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Execute() error = %v, want %v", err, domain.ErrNotFound)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}

	wantCalls := []string{
		"device.get_by_id",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestRevokeDeviceExecuteRecreatesPeerWhenUpdateFails(t *testing.T) {
	callLog := make([]string, 0)
	updateErr := errors.New("update failed")

	deviceRepository := &fakeDeviceRepository{
		callLog: &callLog,
		getByIDDevice: &domain.Device{
			ID:         100,
			UserID:     42,
			PublicKey:  "public-key",
			AssignedIP: "10.67.0.2",
			Status:     domain.DeviceStatusActive,
		},
		updateErr: updateErr,
	}
	transport := &fakeVPNTransport{callLog: &callLog}

	useCase := NewRevokeDeviceUseCase(
		&fakeUserRepository{
			callLog: &callLog,
			user: &domain.User{
				ID:     42,
				Status: domain.UserStatusActive,
			},
		},
		deviceRepository,
		transport,
	)

	result, err := useCase.Execute(context.Background(), RevokeDeviceInput{
		UserID:   42,
		DeviceID: 100,
	})
	if !errors.Is(err, updateErr) {
		t.Fatalf("Execute() error = %v, want %v", err, updateErr)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}

	if transport.removePeerCalls != 1 {
		t.Fatalf("RemovePeer calls = %d, want %d", transport.removePeerCalls, 1)
	}

	if transport.createPeerCalls != 1 {
		t.Fatalf("CreatePeer calls = %d, want %d", transport.createPeerCalls, 1)
	}

	if got := transport.createPeerInput; got.PublicKey != "public-key" || got.AssignedIP != "10.67.0.2" {
		t.Fatalf("CreatePeer input = %#v, want public-key/10.67.0.2", got)
	}

	wantCalls := []string{
		"device.get_by_id",
		"user.get_by_id",
		"transport.remove_peer",
		"device.update",
		"transport.create_peer",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}
