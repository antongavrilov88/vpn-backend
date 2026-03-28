-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE,
    username TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_status_check CHECK (status IN ('active', 'blocked', 'deleted'))
);

CREATE INDEX idx_users_username ON users (username);

CREATE TABLE devices (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    public_key TEXT NOT NULL,
    encrypted_private_key TEXT NOT NULL,
    assigned_ip INET NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    CONSTRAINT devices_status_check CHECK (status IN ('active', 'revoked')),
    CONSTRAINT devices_revoked_at_check CHECK (
        (status = 'active' AND revoked_at IS NULL) OR
        (status = 'revoked' AND revoked_at IS NOT NULL)
    ),
    CONSTRAINT devices_public_key_unique UNIQUE (public_key),
    CONSTRAINT devices_assigned_ip_unique UNIQUE (assigned_ip)
);

CREATE INDEX idx_devices_user_id ON devices (user_id);
CREATE INDEX idx_devices_user_id_status ON devices (user_id, status);

CREATE TABLE subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    plan_code TEXT NOT NULL,
    status TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    is_lifetime BOOLEAN NOT NULL DEFAULT FALSE,
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT subscriptions_status_check CHECK (status IN ('pending', 'active', 'expired', 'canceled')),
    CONSTRAINT subscriptions_source_check CHECK (source IN ('manual', 'telegram', 'admin', 'system')),
    CONSTRAINT subscriptions_lifetime_expires_check CHECK (
        (is_lifetime = TRUE AND expires_at IS NULL) OR
        (is_lifetime = FALSE AND expires_at IS NOT NULL)
    )
);

CREATE INDEX idx_subscriptions_user_id ON subscriptions (user_id);
CREATE INDEX idx_subscriptions_user_id_status ON subscriptions (user_id, status);
CREATE INDEX idx_subscriptions_expires_at ON subscriptions (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE subscriptions;
DROP TABLE devices;
DROP TABLE users;
-- +goose StatementEnd
