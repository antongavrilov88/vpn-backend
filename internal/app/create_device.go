package app

import (
	"context"
	"errors"
	"time"

	"vpn-backend/internal/domain"
)

const maxCreateDeviceAttempts = 3

type CreateDeviceUseCase struct {
	userRepository         domain.UserRepository
	deviceRepository       domain.DeviceRepository
	subscriptionRepository domain.SubscriptionRepository
	transport              domain.VPNTransport
	keyGenerator           domain.KeyGenerator
	privateKeyCipher       domain.PrivateKeyCipher
	ipAllocator            domain.IPAllocator
}

type CreateDeviceInput struct {
	UserID int64
	Name   string
}

type CreateDeviceResult struct {
	Device       *domain.Device
	ClientConfig string
}

func NewCreateDeviceUseCase(
	userRepository domain.UserRepository,
	deviceRepository domain.DeviceRepository,
	subscriptionRepository domain.SubscriptionRepository,
	transport domain.VPNTransport,
	keyGenerator domain.KeyGenerator,
	privateKeyCipher domain.PrivateKeyCipher,
	ipAllocator domain.IPAllocator,
) *CreateDeviceUseCase {
	return &CreateDeviceUseCase{
		userRepository:         userRepository,
		deviceRepository:       deviceRepository,
		subscriptionRepository: subscriptionRepository,
		transport:              transport,
		keyGenerator:           keyGenerator,
		privateKeyCipher:       privateKeyCipher,
		ipAllocator:            ipAllocator,
	}
}

func (uc *CreateDeviceUseCase) Execute(ctx context.Context, input CreateDeviceInput) (*CreateDeviceResult, error) {
	user, err := uc.userRepository.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	if user.Status == domain.UserStatusBlocked {
		return nil, domain.ErrUserBlocked
	}

	if user.Status == domain.UserStatusDeleted {
		return nil, domain.ErrUserDeleted
	}

	if uc.subscriptionRepository != nil {
		if _, err := uc.subscriptionRepository.GetActiveByUserID(ctx, input.UserID); err != nil {
			return nil, err
		}
	}

	keyPair, err := uc.keyGenerator.Generate()
	if err != nil {
		return nil, err
	}

	_, err = uc.deviceRepository.GetByPublicKey(ctx, keyPair.PublicKey)
	if err == nil {
		return nil, domain.ErrDeviceExists
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	encryptedPrivateKey, err := uc.privateKeyCipher.Encrypt(ctx, keyPair.PrivateKey)
	if err != nil {
		return nil, err
	}

	device := domain.Device{
		UserID:              input.UserID,
		Name:                input.Name,
		PublicKey:           keyPair.PublicKey,
		EncryptedPrivateKey: encryptedPrivateKey,
		Status:              domain.DeviceStatusActive,
	}

	createdDevice, err := uc.createDeviceRecord(ctx, device)
	if err != nil {
		return nil, err
	}

	peerInput := domain.CreatePeerInput{
		PublicKey:  createdDevice.PublicKey,
		AssignedIP: createdDevice.AssignedIP,
	}

	if _, err := uc.transport.CreatePeer(ctx, peerInput); err != nil {
		uc.revokeDeviceOnFailure(ctx, createdDevice)

		return nil, err
	}

	clientConfig, err := uc.transport.BuildClientConfig(ctx, domain.BuildClientConfigInput{
		DeviceName:       input.Name,
		ClientPrivateKey: keyPair.PrivateKey,
		ClientAddress:    createdDevice.AssignedIP,
	})
	if err != nil {
		_ = uc.transport.RemovePeer(ctx, domain.RemovePeerInput{
			PublicKey:  createdDevice.PublicKey,
			AssignedIP: createdDevice.AssignedIP,
		})
		uc.revokeDeviceOnFailure(ctx, createdDevice)

		return nil, err
	}

	return &CreateDeviceResult{
		Device:       createdDevice,
		ClientConfig: clientConfig,
	}, nil
}

func (uc *CreateDeviceUseCase) createDeviceRecord(ctx context.Context, device domain.Device) (*domain.Device, error) {
	for attempt := 0; attempt < maxCreateDeviceAttempts; attempt++ {
		assignedIP, err := uc.ipAllocator.AllocateNext(ctx)
		if err != nil {
			return nil, err
		}

		device.AssignedIP = assignedIP

		createdDevice, err := uc.deviceRepository.Create(ctx, device)
		if err == nil {
			return createdDevice, nil
		}

		if !errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
	}

	return nil, domain.ErrConflict
}

func (uc *CreateDeviceUseCase) revokeDeviceOnFailure(ctx context.Context, device *domain.Device) {
	revokedAt := time.Now().UTC()
	device.Status = domain.DeviceStatusRevoked
	device.RevokedAt = &revokedAt

	if _, err := uc.deviceRepository.Update(ctx, *device); err != nil {
		return
	}
}
