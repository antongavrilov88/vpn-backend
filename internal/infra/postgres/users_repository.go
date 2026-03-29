package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"vpn-backend/internal/domain"
)

type UserRepository struct {
	db *pgxpool.Pool
}

var _ domain.UserRepository = (*UserRepository)(nil)

const userColumns = `
id,
telegram_id,
username,
status,
created_at,
updated_at
`

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	const query = `
SELECT ` + userColumns + `
FROM users
WHERE id = $1
`

	user, err := scanUser(
		r.db.QueryRow(ctx, query, id),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return user, nil
}

func (r *UserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	const query = `
SELECT ` + userColumns + `
FROM users
WHERE telegram_id = $1
`

	user, err := scanUser(
		r.db.QueryRow(ctx, query, telegramID),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return user, nil
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) (*domain.User, error) {
	const query = `
INSERT INTO users (telegram_id, username, status)
VALUES ($1, $2, $3)
RETURNING ` + userColumns

	createdUser, err := scanUser(
		r.db.QueryRow(ctx, query, user.TelegramID, user.Username, user.Status),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return createdUser, nil
}

func (r *UserRepository) Update(ctx context.Context, user domain.User) (*domain.User, error) {
	const query = `
UPDATE users
SET telegram_id = $2,
    username = $3,
    status = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING ` + userColumns

	updatedUser, err := scanUser(
		r.db.QueryRow(ctx, query, user.ID, user.TelegramID, user.Username, user.Status),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return updatedUser, nil
}

type userScanner interface {
	Scan(dest ...interface{}) error
}

func scanUser(scanner userScanner) (*domain.User, error) {
	var user domain.User

	if err := scanner.Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &user, nil
}
