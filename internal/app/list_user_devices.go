package app

import (
	"context"
	"time"

	"vpn-backend/internal/domain"
)

type ListUserDevicesUseCase struct {
	userRepository   domain.UserRepository
	deviceRepository domain.DeviceRepository
}

type ListUserDevicesInput struct {
	CallerUserID int64
	UserID       int64
}

type ListUserDevicesResult struct {
	Devices []ListUserDevicesItem
}

type ListUserDevicesItem struct {
	ID         int64
	Name       string
	AssignedIP string
	Status     domain.DeviceStatus
	CreatedAt  time.Time
	RevokedAt  *time.Time
}

func NewListUserDevicesUseCase(
	userRepository domain.UserRepository,
	deviceRepository domain.DeviceRepository,
) *ListUserDevicesUseCase {
	return &ListUserDevicesUseCase{
		userRepository:   userRepository,
		deviceRepository: deviceRepository,
	}
}

func (uc *ListUserDevicesUseCase) Execute(ctx context.Context, input ListUserDevicesInput) (*ListUserDevicesResult, error) {
	if input.CallerUserID != input.UserID {
		return nil, domain.ErrNotFound
	}

	if _, err := loadAccessibleUser(ctx, uc.userRepository, input.UserID); err != nil {
		return nil, err
	}

	devices, err := uc.deviceRepository.ListByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	items := make([]ListUserDevicesItem, 0, len(devices))
	for _, device := range devices {
		items = append(items, ListUserDevicesItem{
			ID:         device.ID,
			Name:       device.Name,
			AssignedIP: device.AssignedIP,
			Status:     device.Status,
			CreatedAt:  device.CreatedAt,
			RevokedAt:  device.RevokedAt,
		})
	}

	return &ListUserDevicesResult{Devices: items}, nil
}
