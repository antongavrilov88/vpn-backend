package domain

import "context"

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*User, error)
	Create(ctx context.Context, user User) (*User, error)
	Update(ctx context.Context, user User) (*User, error)
}

type DeviceRepository interface {
	GetByID(ctx context.Context, id int64) (*Device, error)
	GetByPublicKey(ctx context.Context, publicKey string) (*Device, error)
	ListByUserID(ctx context.Context, userID int64) ([]Device, error)
	Create(ctx context.Context, device Device) (*Device, error)
	Update(ctx context.Context, device Device) (*Device, error)
}

type SubscriptionRepository interface {
	GetByID(ctx context.Context, id int64) (*Subscription, error)
	ListByUserID(ctx context.Context, userID int64) ([]Subscription, error)
	GetActiveByUserID(ctx context.Context, userID int64) (*Subscription, error)
	Create(ctx context.Context, subscription Subscription) (*Subscription, error)
	Update(ctx context.Context, subscription Subscription) (*Subscription, error)
}

type PromoCodeRepository interface {
	GetByCode(ctx context.Context, code string) (*PromoCode, error)
	Create(ctx context.Context, promoCode PromoCode) (*PromoCode, error)
	Update(ctx context.Context, promoCode PromoCode) (*PromoCode, error)
}

type PromoCodeUsageRepository interface {
	ListByUserID(ctx context.Context, userID int64) ([]PromoCodeUsage, error)
	HasUsage(ctx context.Context, promoCodeID, userID int64) (bool, error)
	Create(ctx context.Context, usage PromoCodeUsage) (*PromoCodeUsage, error)
}

type AuditLogRepository interface {
	Create(ctx context.Context, auditLog AuditLog) (*AuditLog, error)
	ListByEntity(ctx context.Context, entityType AuditEntityType, entityID *int64) ([]AuditLog, error)
}
