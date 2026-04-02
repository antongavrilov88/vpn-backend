package app

import (
	"context"
	"errors"
	"fmt"

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
	clientConfigBuilder    domain.ClientConfigBuilder
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
	clientConfigBuilder domain.ClientConfigBuilder,
) *CreateDeviceUseCase {
	return &CreateDeviceUseCase{
		userRepository:         userRepository,
		deviceRepository:       deviceRepository,
		subscriptionRepository: subscriptionRepository,
		transport:              transport,
		keyGenerator:           keyGenerator,
		privateKeyCipher:       privateKeyCipher,
		ipAllocator:            ipAllocator,
		clientConfigBuilder:    clientConfigBuilder,
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

	if uc.clientConfigBuilder == nil {
		return nil, fmt.Errorf("client config builder is not configured")
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
	for attempt := 0; attempt < maxCreateDeviceAttempts; attempt++ {
		assignedIP, err := uc.ipAllocator.AllocateNext(ctx)
		if err != nil {
			return nil, err
		}

		peerInput := domain.CreatePeerInput{
			PublicKey:  keyPair.PublicKey,
			AssignedIP: assignedIP,
		}

		if _, err := uc.transport.CreatePeer(ctx, peerInput); err != nil {
			return nil, err
		}

		clientConfig, err := uc.clientConfigBuilder.Build(ctx, domain.BuildClientConfigInput{
			DeviceName:       input.Name,
			ClientPrivateKey: keyPair.PrivateKey,
			ClientAddress:    assignedIP,
		})
		if err != nil {
			return nil, uc.cleanupPeerFailure(ctx, peerInput, err)
		}

		encryptedPrivateKey, err := uc.privateKeyCipher.Encrypt(ctx, keyPair.PrivateKey)
		if err != nil {
			return nil, uc.cleanupPeerFailure(ctx, peerInput, err)
		}

		device := domain.Device{
			UserID:              input.UserID,
			Name:                input.Name,
			PublicKey:           keyPair.PublicKey,
			EncryptedPrivateKey: encryptedPrivateKey,
			AssignedIP:          assignedIP,
			Status:              domain.DeviceStatusActive,
		}

		createdDevice, err := uc.deviceRepository.Create(ctx, device)
		if err == nil {
			return &CreateDeviceResult{
				Device:       createdDevice,
				ClientConfig: clientConfig,
			}, nil
		}

		err = uc.cleanupPeerFailure(ctx, peerInput, err)
		if !errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
	}

	return nil, domain.ErrConflict
}

func (uc *CreateDeviceUseCase) cleanupPeerFailure(ctx context.Context, input domain.CreatePeerInput, cause error) error {
	if err := uc.transport.RemovePeer(ctx, domain.RemovePeerInput{
		PublicKey:  input.PublicKey,
		AssignedIP: input.AssignedIP,
	}); err != nil {
		return fmt.Errorf("%w: remove peer compensation failed: %v", cause, err)
	}

	return cause
}
