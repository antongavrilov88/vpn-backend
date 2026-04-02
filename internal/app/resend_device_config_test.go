package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"vpn-backend/internal/domain"
)

func TestResendDeviceConfigExecuteHappyPath(t *testing.T) {
	callLog := make([]string, 0)

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
			ID:                  100,
			UserID:              42,
			Name:                "Dad Phone",
			EncryptedPrivateKey: "encrypted-private-key",
			AssignedIP:          "10.67.0.2",
			Status:              domain.DeviceStatusActive,
		},
	}
	privateKeyCipher := &fakePrivateKeyCipher{
		callLog:       &callLog,
		decryptResult: "private-key",
	}
	clientConfigBuilder := &fakeClientConfigBuilder{
		callLog: &callLog,
		result:  "[Interface]\nPrivateKey = private-key\n",
	}

	useCase := NewResendDeviceConfigUseCase(
		userRepository,
		deviceRepository,
		privateKeyCipher,
		clientConfigBuilder,
	)

	result, err := useCase.Execute(context.Background(), ResendDeviceConfigInput{
		UserID:   42,
		DeviceID: 100,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil || result.Device == nil {
		t.Fatal("Execute() result/device = nil, want non-nil")
	}

	if result.Device.ID != 100 {
		t.Fatalf("Device.ID = %d, want %d", result.Device.ID, 100)
	}

	if result.ClientConfig != "[Interface]\nPrivateKey = private-key\n" {
		t.Fatalf("ClientConfig = %q, want expected config", result.ClientConfig)
	}

	if got := privateKeyCipher.decryptCiphertext; got != "encrypted-private-key" {
		t.Fatalf("Decrypt ciphertext = %q, want %q", got, "encrypted-private-key")
	}

	if got := clientConfigBuilder.input; got.DeviceName != "Dad Phone" || got.ClientPrivateKey != "private-key" || got.ClientAddress != "10.67.0.2" {
		t.Fatalf("Build input = %#v, want expected values", got)
	}

	wantCalls := []string{
		"device.get_by_id",
		"user.get_by_id",
		"cipher.decrypt",
		"config_builder.build",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestResendDeviceConfigExecuteReturnsNotFoundWhenDeviceMissing(t *testing.T) {
	userRepository := &fakeUserRepository{}
	deviceRepository := &fakeDeviceRepository{
		getByIDErr: domain.ErrNotFound,
	}
	privateKeyCipher := &fakePrivateKeyCipher{}
	clientConfigBuilder := &fakeClientConfigBuilder{}

	useCase := NewResendDeviceConfigUseCase(
		userRepository,
		deviceRepository,
		privateKeyCipher,
		clientConfigBuilder,
	)

	result, err := useCase.Execute(context.Background(), ResendDeviceConfigInput{
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

func TestResendDeviceConfigExecuteReturnsDecryptError(t *testing.T) {
	callLog := make([]string, 0)
	decryptErr := errors.New("decrypt failed")

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
			ID:                  100,
			UserID:              42,
			Name:                "Dad Phone",
			EncryptedPrivateKey: "encrypted-private-key",
			AssignedIP:          "10.67.0.2",
			Status:              domain.DeviceStatusActive,
		},
	}
	privateKeyCipher := &fakePrivateKeyCipher{
		callLog:    &callLog,
		decryptErr: decryptErr,
	}
	clientConfigBuilder := &fakeClientConfigBuilder{
		callLog: &callLog,
	}

	useCase := NewResendDeviceConfigUseCase(
		userRepository,
		deviceRepository,
		privateKeyCipher,
		clientConfigBuilder,
	)

	result, err := useCase.Execute(context.Background(), ResendDeviceConfigInput{
		UserID:   42,
		DeviceID: 100,
	})
	if !errors.Is(err, decryptErr) {
		t.Fatalf("Execute() error = %v, want %v", err, decryptErr)
	}

	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}

	if clientConfigBuilder.input.DeviceName != "" {
		t.Fatalf("Build should not be called, got input %#v", clientConfigBuilder.input)
	}

	wantCalls := []string{
		"device.get_by_id",
		"user.get_by_id",
		"cipher.decrypt",
	}
	if !reflect.DeepEqual(callLog, wantCalls) {
		t.Fatalf("call log = %#v, want %#v", callLog, wantCalls)
	}
}

func TestResendDeviceConfigExecuteReturnsNotFoundForForeignDevice(t *testing.T) {
	callLog := make([]string, 0)

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
			ID:                  100,
			UserID:              99,
			Name:                "Dad Phone",
			EncryptedPrivateKey: "encrypted-private-key",
			AssignedIP:          "10.67.0.2",
			Status:              domain.DeviceStatusActive,
		},
	}

	useCase := NewResendDeviceConfigUseCase(
		userRepository,
		deviceRepository,
		&fakePrivateKeyCipher{callLog: &callLog},
		&fakeClientConfigBuilder{callLog: &callLog},
	)

	result, err := useCase.Execute(context.Background(), ResendDeviceConfigInput{
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
