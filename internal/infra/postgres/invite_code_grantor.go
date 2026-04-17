package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"vpn-backend/internal/domain"
)

const closedBetaInvitePlanCode = "closed_beta_invite"

type InviteCodeGrantor struct {
	db *pgxpool.Pool
}

func NewInviteCodeGrantor(db *pgxpool.Pool) *InviteCodeGrantor {
	return &InviteCodeGrantor{db: db}
}

func (g *InviteCodeGrantor) ApplyInviteCode(ctx context.Context, userID int64, code string, now time.Time) (*domain.Subscription, error) {
	tx, err := g.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	promoCode, err := loadPromoCodeForUpdate(ctx, tx, code)
	if err != nil {
		return nil, err
	}

	if err := validateAccessPromoCode(promoCode, now); err != nil {
		return nil, err
	}

	activeSubscription, err := loadActiveSubscriptionForUpdate(ctx, tx, userID)
	if err != nil && !errors.Is(err, domain.ErrSubscriptionMiss) {
		return nil, err
	}

	if shouldSkipInviteCodeConsumption(activeSubscription) {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return activeSubscription, nil
	}

	if err := insertPromoCodeUsage(ctx, tx, promoCode.ID, userID, now); err != nil {
		return nil, err
	}

	if err := incrementPromoCodeUsage(ctx, tx, promoCode.ID); err != nil {
		return nil, err
	}

	subscription, err := grantSubscriptionForInviteCode(ctx, tx, userID, promoCode, activeSubscription, now)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return subscription, nil
}

func shouldSkipInviteCodeConsumption(activeSubscription *domain.Subscription) bool {
	return activeSubscription != nil && activeSubscription.IsLifetime
}

func loadPromoCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (*domain.PromoCode, error) {
	const query = `
SELECT
    id,
    code,
    type,
    duration_days,
    percent_off,
    max_usages,
    current_usages,
    is_active,
    valid_from,
    valid_until,
    created_at
FROM promo_codes
WHERE LOWER(code) = LOWER($1)
FOR UPDATE
`

	var promoCode domain.PromoCode
	if err := tx.QueryRow(ctx, query, strings.TrimSpace(code)).Scan(
		&promoCode.ID,
		&promoCode.Code,
		&promoCode.Type,
		&promoCode.DurationDays,
		&promoCode.PercentOff,
		&promoCode.MaxUsages,
		&promoCode.CurrentUsages,
		&promoCode.IsActive,
		&promoCode.ValidFrom,
		&promoCode.ValidUntil,
		&promoCode.CreatedAt,
	); err != nil {
		return nil, mapError(err)
	}

	return &promoCode, nil
}

func validateAccessPromoCode(promoCode *domain.PromoCode, now time.Time) error {
	if promoCode == nil {
		return domain.ErrNotFound
	}

	if !promoCode.IsActive {
		return domain.ErrPromoCodeInactive
	}

	if promoCode.ValidFrom != nil && now.Before(*promoCode.ValidFrom) {
		return domain.ErrPromoCodeInactive
	}

	if promoCode.ValidUntil != nil && now.After(*promoCode.ValidUntil) {
		return domain.ErrPromoCodeInactive
	}

	if promoCode.MaxUsages != nil && promoCode.CurrentUsages >= *promoCode.MaxUsages {
		return domain.ErrPromoCodeLimit
	}

	switch promoCode.Type {
	case domain.PromoCodeTypeDurationDays, domain.PromoCodeTypeLifetimeAccess:
		return nil
	default:
		return domain.ErrPromoCodeAccess
	}
}

func insertPromoCodeUsage(ctx context.Context, tx pgx.Tx, promoCodeID, userID int64, now time.Time) error {
	const query = `
INSERT INTO promo_code_usages (promo_code_id, user_id, applied_at)
VALUES ($1, $2, $3)
`

	_, err := tx.Exec(ctx, query, promoCodeID, userID, now)
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return domain.ErrPromoCodeUsed
	}

	return err
}

func incrementPromoCodeUsage(ctx context.Context, tx pgx.Tx, promoCodeID int64) error {
	const query = `
UPDATE promo_codes
SET current_usages = current_usages + 1
WHERE id = $1
`

	_, err := tx.Exec(ctx, query, promoCodeID)
	return err
}

func loadActiveSubscriptionForUpdate(ctx context.Context, tx pgx.Tx, userID int64) (*domain.Subscription, error) {
	const query = `
SELECT
    id,
    user_id,
    plan_code,
    status,
    starts_at,
    expires_at,
    is_lifetime,
    source,
    created_at,
    updated_at
FROM subscriptions
WHERE user_id = $1
  AND status = 'active'
  AND starts_at <= NOW()
  AND (is_lifetime = TRUE OR expires_at >= NOW())
ORDER BY is_lifetime DESC, expires_at DESC NULLS LAST, created_at DESC, id DESC
LIMIT 1
FOR UPDATE
`

	var subscription domain.Subscription
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&subscription.ID,
		&subscription.UserID,
		&subscription.PlanCode,
		&subscription.Status,
		&subscription.StartsAt,
		&subscription.ExpiresAt,
		&subscription.IsLifetime,
		&subscription.Source,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSubscriptionMiss
		}

		return nil, err
	}

	return &subscription, nil
}

func grantSubscriptionForInviteCode(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	promoCode *domain.PromoCode,
	activeSubscription *domain.Subscription,
	now time.Time,
) (*domain.Subscription, error) {
	switch promoCode.Type {
	case domain.PromoCodeTypeDurationDays:
		if activeSubscription != nil && activeSubscription.IsLifetime {
			return activeSubscription, nil
		}

		expiresAt := now.AddDate(0, 0, int(*promoCode.DurationDays))
		if activeSubscription != nil && activeSubscription.ExpiresAt != nil && activeSubscription.ExpiresAt.After(now) {
			expiresAt = activeSubscription.ExpiresAt.AddDate(0, 0, int(*promoCode.DurationDays))
		}

		if activeSubscription != nil {
			activeSubscription.PlanCode = closedBetaInvitePlanCode
			activeSubscription.Status = domain.SubscriptionStatusActive
			activeSubscription.ExpiresAt = &expiresAt
			activeSubscription.IsLifetime = false
			activeSubscription.Source = domain.SubscriptionSourceTelegram
			return updateSubscription(ctx, tx, *activeSubscription)
		}

		return createSubscription(ctx, tx, domain.Subscription{
			UserID:     userID,
			PlanCode:   closedBetaInvitePlanCode,
			Status:     domain.SubscriptionStatusActive,
			StartsAt:   now,
			ExpiresAt:  &expiresAt,
			IsLifetime: false,
			Source:     domain.SubscriptionSourceTelegram,
		})
	case domain.PromoCodeTypeLifetimeAccess:
		if activeSubscription != nil {
			if activeSubscription.IsLifetime {
				return activeSubscription, nil
			}

			activeSubscription.PlanCode = closedBetaInvitePlanCode
			activeSubscription.Status = domain.SubscriptionStatusActive
			activeSubscription.ExpiresAt = nil
			activeSubscription.IsLifetime = true
			activeSubscription.Source = domain.SubscriptionSourceTelegram
			return updateSubscription(ctx, tx, *activeSubscription)
		}

		return createSubscription(ctx, tx, domain.Subscription{
			UserID:     userID,
			PlanCode:   closedBetaInvitePlanCode,
			Status:     domain.SubscriptionStatusActive,
			StartsAt:   now,
			ExpiresAt:  nil,
			IsLifetime: true,
			Source:     domain.SubscriptionSourceTelegram,
		})
	default:
		return nil, domain.ErrPromoCodeAccess
	}
}

func createSubscription(ctx context.Context, tx pgx.Tx, subscription domain.Subscription) (*domain.Subscription, error) {
	const query = `
INSERT INTO subscriptions (
    user_id,
    plan_code,
    status,
    starts_at,
    expires_at,
    is_lifetime,
    source
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    id,
    user_id,
    plan_code,
    status,
    starts_at,
    expires_at,
    is_lifetime,
    source,
    created_at,
    updated_at
`

	var created domain.Subscription
	if err := tx.QueryRow(
		ctx,
		query,
		subscription.UserID,
		subscription.PlanCode,
		subscription.Status,
		subscription.StartsAt,
		subscription.ExpiresAt,
		subscription.IsLifetime,
		subscription.Source,
	).Scan(
		&created.ID,
		&created.UserID,
		&created.PlanCode,
		&created.Status,
		&created.StartsAt,
		&created.ExpiresAt,
		&created.IsLifetime,
		&created.Source,
		&created.CreatedAt,
		&created.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &created, nil
}

func updateSubscription(ctx context.Context, tx pgx.Tx, subscription domain.Subscription) (*domain.Subscription, error) {
	const query = `
UPDATE subscriptions
SET plan_code = $2,
    status = $3,
    starts_at = $4,
    expires_at = $5,
    is_lifetime = $6,
    source = $7,
    updated_at = NOW()
WHERE id = $1
RETURNING
    id,
    user_id,
    plan_code,
    status,
    starts_at,
    expires_at,
    is_lifetime,
    source,
    created_at,
    updated_at
`

	var updated domain.Subscription
	if err := tx.QueryRow(
		ctx,
		query,
		subscription.ID,
		subscription.PlanCode,
		subscription.Status,
		subscription.StartsAt,
		subscription.ExpiresAt,
		subscription.IsLifetime,
		subscription.Source,
	).Scan(
		&updated.ID,
		&updated.UserID,
		&updated.PlanCode,
		&updated.Status,
		&updated.StartsAt,
		&updated.ExpiresAt,
		&updated.IsLifetime,
		&updated.Source,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &updated, nil
}
