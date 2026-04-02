package app

import (
	"context"
	"fmt"
	"time"

	"vpn-backend/internal/domain"
)

type RevokeDeviceUseCase struct {
	userRepository   domain.UserRepository
	deviceRepository domain.DeviceRepository
	transport        domain.VPNTransport
	now              func() time.Time
}

type RevokeDeviceInput struct {
	UserID   int64
	DeviceID int64
}

type RevokeDeviceResult struct {
	Device *domain.Device
}

func NewRevokeDeviceUseCase(
	userRepository domain.UserRepository,
	deviceRepository domain.DeviceRepository,
	transport domain.VPNTransport,
) *RevokeDeviceUseCase {
	return &RevokeDeviceUseCase{
		userRepository:   userRepository,
		deviceRepository: deviceRepository,
		transport:        transport,
		now:              time.Now,
	}
}

func (uc *RevokeDeviceUseCase) Execute(ctx context.Context, input RevokeDeviceInput) (*RevokeDeviceResult, error) {
	device, err := uc.deviceRepository.GetByID(ctx, input.DeviceID)
	if err != nil {
		return nil, err
	}

	if device.UserID != input.UserID {
		return nil, domain.ErrNotFound
	}

	if _, err := loadAccessibleUser(ctx, uc.userRepository, input.UserID); err != nil {
		return nil, err
	}

	if device.Status == domain.DeviceStatusRevoked {
		return nil, domain.ErrDeviceRevoked
	}

	if err := uc.transport.RemovePeer(ctx, domain.RemovePeerInput{
		PublicKey:  device.PublicKey,
		AssignedIP: device.AssignedIP,
	}); err != nil {
		return nil, err
	}

	revokedAt := uc.now().UTC()
	device.Status = domain.DeviceStatusRevoked
	device.RevokedAt = &revokedAt

	updatedDevice, err := uc.deviceRepository.Update(ctx, *device)
	if err != nil {
		return nil, uc.restorePeerAfterUpdateFailure(ctx, device, err)
	}

	return &RevokeDeviceResult{Device: updatedDevice}, nil
}

func (uc *RevokeDeviceUseCase) restorePeerAfterUpdateFailure(ctx context.Context, device *domain.Device, cause error) error {
	if _, err := uc.transport.CreatePeer(ctx, domain.CreatePeerInput{
		PublicKey:  device.PublicKey,
		AssignedIP: device.AssignedIP,
	}); err != nil {
		return fmt.Errorf("%w: recreate peer compensation failed: %v", cause, err)
	}

	return cause
}
