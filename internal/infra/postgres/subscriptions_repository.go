package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vpn-backend/internal/domain"
)

type SubscriptionRepository struct {
	db *pgxpool.Pool
}

var _ domain.SubscriptionRepository = (*SubscriptionRepository)(nil)

const subscriptionColumns = `
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

func NewSubscriptionRepository(db *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id int64) (*domain.Subscription, error) {
	const query = `
SELECT ` + subscriptionColumns + `
FROM subscriptions
WHERE id = $1
`

	subscription, err := scanSubscription(
		r.db.QueryRow(ctx, query, id),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return subscription, nil
}

func (r *SubscriptionRepository) ListByUserID(ctx context.Context, userID int64) ([]domain.Subscription, error) {
	const query = `
SELECT ` + subscriptionColumns + `
FROM subscriptions
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	subscriptions := make([]domain.Subscription, 0)
	for rows.Next() {
		subscription, scanErr := scanSubscription(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		subscriptions = append(subscriptions, *subscription)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (r *SubscriptionRepository) GetActiveByUserID(ctx context.Context, userID int64) (*domain.Subscription, error) {
	const query = `
SELECT ` + subscriptionColumns + `
FROM subscriptions
WHERE user_id = $1
  AND status = 'active'
  AND starts_at <= NOW()
  AND (is_lifetime = TRUE OR expires_at >= NOW())
ORDER BY is_lifetime DESC, expires_at DESC NULLS LAST, created_at DESC, id DESC
LIMIT 1
`

	subscription, err := scanSubscription(
		r.db.QueryRow(ctx, query, userID),
	)
	if err != nil {
		mappedErr := mapError(err)
		if mappedErr == domain.ErrNotFound {
			return nil, domain.ErrSubscriptionMiss
		}

		return nil, mappedErr
	}

	return subscription, nil
}

func (r *SubscriptionRepository) Create(ctx context.Context, subscription domain.Subscription) (*domain.Subscription, error) {
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
RETURNING ` + subscriptionColumns

	createdSubscription, err := scanSubscription(
		r.db.QueryRow(
			ctx,
			query,
			subscription.UserID,
			subscription.PlanCode,
			subscription.Status,
			subscription.StartsAt,
			subscription.ExpiresAt,
			subscription.IsLifetime,
			subscription.Source,
		),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return createdSubscription, nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, subscription domain.Subscription) (*domain.Subscription, error) {
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
RETURNING ` + subscriptionColumns

	updatedSubscription, err := scanSubscription(
		r.db.QueryRow(
			ctx,
			query,
			subscription.ID,
			subscription.PlanCode,
			subscription.Status,
			subscription.StartsAt,
			subscription.ExpiresAt,
			subscription.IsLifetime,
			subscription.Source,
		),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return updatedSubscription, nil
}

type subscriptionScanner interface {
	Scan(dest ...interface{}) error
}

func scanSubscription(scanner subscriptionScanner) (*domain.Subscription, error) {
	var subscription domain.Subscription

	if err := scanner.Scan(
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
		return nil, err
	}

	return &subscription, nil
}

var _ subscriptionScanner = (pgx.Row)(nil)
