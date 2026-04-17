package domain

import (
	"encoding/json"
	"time"
)

type UserStatus string

const (
	UserStatusActive  UserStatus = "active"
	UserStatusBlocked UserStatus = "blocked"
	UserStatusDeleted UserStatus = "deleted"
)

type DeviceStatus string

const (
	DeviceStatusActive  DeviceStatus = "active"
	DeviceStatusRevoked DeviceStatus = "revoked"
)

type SubscriptionStatus string

const (
	SubscriptionStatusPending  SubscriptionStatus = "pending"
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusExpired  SubscriptionStatus = "expired"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
)

type SubscriptionSource string

const (
	SubscriptionSourceManual   SubscriptionSource = "manual"
	SubscriptionSourceTelegram SubscriptionSource = "telegram"
	SubscriptionSourceAdmin    SubscriptionSource = "admin"
	SubscriptionSourceSystem   SubscriptionSource = "system"
)

type PromoCodeType string

const (
	PromoCodeTypeDurationDays   PromoCodeType = "duration_days"
	PromoCodeTypePercentOff     PromoCodeType = "percent_off"
	PromoCodeTypeLifetimeAccess PromoCodeType = "lifetime_access"
)

type AccessDenialReason string

const (
	AccessDenialReasonNone               AccessDenialReason = ""
	AccessDenialReasonInviteCodeRequired AccessDenialReason = "invite_code_required"
	AccessDenialReasonExpired            AccessDenialReason = "expired"
	AccessDenialReasonPending            AccessDenialReason = "pending"
	AccessDenialReasonCanceled           AccessDenialReason = "canceled"
	AccessDenialReasonUserBlocked        AccessDenialReason = "user_blocked"
	AccessDenialReasonUserDeleted        AccessDenialReason = "user_deleted"
)

type AuditActorType string

const (
	AuditActorTypeSystem  AuditActorType = "system"
	AuditActorTypeUser    AuditActorType = "user"
	AuditActorTypeAdmin   AuditActorType = "admin"
	AuditActorTypeSupport AuditActorType = "support"
)

type AuditEntityType string

const (
	AuditEntityTypeSystem         AuditEntityType = "system"
	AuditEntityTypeUser           AuditEntityType = "user"
	AuditEntityTypeDevice         AuditEntityType = "device"
	AuditEntityTypeSubscription   AuditEntityType = "subscription"
	AuditEntityTypePromoCode      AuditEntityType = "promo_code"
	AuditEntityTypePromoCodeUsage AuditEntityType = "promo_code_usage"
)

type User struct {
	ID         int64
	TelegramID *int64
	Username   string
	Status     UserStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Device struct {
	ID                  int64
	UserID              int64
	Name                string
	PublicKey           string
	EncryptedPrivateKey string
	AssignedIP          string
	Status              DeviceStatus
	CreatedAt           time.Time
	RevokedAt           *time.Time
}

type Subscription struct {
	ID         int64
	UserID     int64
	PlanCode   string
	Status     SubscriptionStatus
	StartsAt   time.Time
	ExpiresAt  *time.Time
	IsLifetime bool
	Source     SubscriptionSource
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type PromoCode struct {
	ID            int64
	Code          string
	Type          PromoCodeType
	DurationDays  *int32
	PercentOff    *int32
	MaxUsages     *int32
	CurrentUsages int32
	IsActive      bool
	ValidFrom     *time.Time
	ValidUntil    *time.Time
	CreatedAt     time.Time
}

type PromoCodeUsage struct {
	ID          int64
	PromoCodeID int64
	UserID      int64
	AppliedAt   time.Time
}

type AuditLog struct {
	ID         int64
	ActorType  AuditActorType
	ActorID    *int64
	Action     string
	EntityType AuditEntityType
	EntityID   *int64
	Payload    json.RawMessage
	CreatedAt  time.Time
}
