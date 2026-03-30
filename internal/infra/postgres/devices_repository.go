package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"vpn-backend/internal/domain"
)

type DeviceRepository struct {
	db *pgxpool.Pool
}

var _ domain.DeviceRepository = (*DeviceRepository)(nil)

const deviceColumns = `
id,
user_id,
name,
public_key,
encrypted_private_key,
assigned_ip,
status,
created_at,
revoked_at
`

func NewDeviceRepository(db *pgxpool.Pool) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) GetByID(ctx context.Context, id int64) (*domain.Device, error) {
	const query = `
SELECT ` + deviceColumns + `
FROM devices
WHERE id = $1
`

	device, err := scanDevice(r.db.QueryRow(ctx, query, id))
	if err != nil {
		return nil, mapError(err)
	}

	return device, nil
}

func (r *DeviceRepository) GetByPublicKey(ctx context.Context, publicKey string) (*domain.Device, error) {
	const query = `
SELECT ` + deviceColumns + `
FROM devices
WHERE public_key = $1
`

	device, err := scanDevice(r.db.QueryRow(ctx, query, publicKey))
	if err != nil {
		return nil, mapError(err)
	}

	return device, nil
}

func (r *DeviceRepository) ListByUserID(ctx context.Context, userID int64) ([]domain.Device, error) {
	const query = `
SELECT ` + deviceColumns + `
FROM devices
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	devices := make([]domain.Device, 0)
	for rows.Next() {
		device, scanErr := scanDevice(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		devices = append(devices, *device)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

func (r *DeviceRepository) Create(ctx context.Context, device domain.Device) (*domain.Device, error) {
	const query = `
INSERT INTO devices (
    user_id,
    name,
    public_key,
    encrypted_private_key,
    assigned_ip,
    status,
    revoked_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING ` + deviceColumns

	createdDevice, err := scanDevice(
		r.db.QueryRow(
			ctx,
			query,
			device.UserID,
			device.Name,
			device.PublicKey,
			device.EncryptedPrivateKey,
			device.AssignedIP,
			device.Status,
			device.RevokedAt,
		),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return createdDevice, nil
}

func (r *DeviceRepository) Update(ctx context.Context, device domain.Device) (*domain.Device, error) {
	const query = `
UPDATE devices
SET name = $2,
    public_key = $3,
    encrypted_private_key = $4,
    assigned_ip = $5,
    status = $6,
    revoked_at = $7
WHERE id = $1
RETURNING ` + deviceColumns

	updatedDevice, err := scanDevice(
		r.db.QueryRow(
			ctx,
			query,
			device.ID,
			device.Name,
			device.PublicKey,
			device.EncryptedPrivateKey,
			device.AssignedIP,
			device.Status,
			device.RevokedAt,
		),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return updatedDevice, nil
}

type deviceScanner interface {
	Scan(dest ...interface{}) error
}

func scanDevice(scanner deviceScanner) (*domain.Device, error) {
	var device domain.Device

	if err := scanner.Scan(
		&device.ID,
		&device.UserID,
		&device.Name,
		&device.PublicKey,
		&device.EncryptedPrivateKey,
		&device.AssignedIP,
		&device.Status,
		&device.CreatedAt,
		&device.RevokedAt,
	); err != nil {
		return nil, err
	}

	return &device, nil
}
