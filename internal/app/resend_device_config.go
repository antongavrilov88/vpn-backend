package app

import (
	"context"
	"fmt"

	"vpn-backend/internal/domain"
)

type ResendDeviceConfigUseCase struct {
	userRepository      domain.UserRepository
	deviceRepository    domain.DeviceRepository
	privateKeyCipher    domain.PrivateKeyCipher
	clientConfigBuilder domain.ClientConfigBuilder
}

type ResendDeviceConfigInput struct {
	UserID   int64
	DeviceID int64
}

type ResendDeviceConfigResult struct {
	Device       *domain.Device
	ClientConfig string
}

func NewResendDeviceConfigUseCase(
	userRepository domain.UserRepository,
	deviceRepository domain.DeviceRepository,
	privateKeyCipher domain.PrivateKeyCipher,
	clientConfigBuilder domain.ClientConfigBuilder,
) *ResendDeviceConfigUseCase {
	return &ResendDeviceConfigUseCase{
		userRepository:      userRepository,
		deviceRepository:    deviceRepository,
		privateKeyCipher:    privateKeyCipher,
		clientConfigBuilder: clientConfigBuilder,
	}
}

func (uc *ResendDeviceConfigUseCase) Execute(ctx context.Context, input ResendDeviceConfigInput) (*ResendDeviceConfigResult, error) {
	device, err := uc.deviceRepository.GetByID(ctx, input.DeviceID)
	if err != nil {
		return nil, err
	}

	if device.UserID != input.UserID {
		return nil, domain.ErrNotFound
	}

	_, err = loadAccessibleUser(ctx, uc.userRepository, input.UserID)
	if err != nil {
		return nil, err
	}

	if device.Status == domain.DeviceStatusRevoked {
		return nil, domain.ErrDeviceRevoked
	}

	if uc.clientConfigBuilder == nil {
		return nil, fmt.Errorf("client config builder is not configured")
	}

	privateKey, err := uc.privateKeyCipher.Decrypt(ctx, device.EncryptedPrivateKey)
	if err != nil {
		return nil, err
	}

	clientConfig, err := uc.clientConfigBuilder.Build(ctx, domain.BuildClientConfigInput{
		DeviceName:       device.Name,
		ClientPrivateKey: privateKey,
		ClientAddress:    device.AssignedIP,
	})
	if err != nil {
		return nil, err
	}

	return &ResendDeviceConfigResult{
		Device:       device,
		ClientConfig: clientConfig,
	}, nil
}
