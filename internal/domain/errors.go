package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrInvalidState      = errors.New("invalid state")
	ErrUserBlocked       = errors.New("user is blocked")
	ErrSubscriptionMiss  = errors.New("active subscription not found")
	ErrDeviceRevoked     = errors.New("device is revoked")
	ErrPromoCodeInactive = errors.New("promo code is inactive")
	ErrPromoCodeUsed     = errors.New("promo code already used")
	ErrPromoCodeLimit    = errors.New("promo code usage limit reached")
)
