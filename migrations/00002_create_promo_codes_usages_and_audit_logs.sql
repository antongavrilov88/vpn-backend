-- +goose Up
-- +goose StatementBegin
CREATE TABLE promo_codes (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    type TEXT NOT NULL,
    duration_days INTEGER,
    percent_off NUMERIC(5,2),
    max_usages INTEGER,
    current_usages INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    valid_from TIMESTAMPTZ,
    valid_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT promo_codes_code_unique UNIQUE (code),
    CONSTRAINT promo_codes_type_check CHECK (type IN ('duration_days', 'percent_off')),
    CONSTRAINT promo_codes_duration_days_check CHECK (duration_days IS NULL OR duration_days > 0),
    CONSTRAINT promo_codes_percent_off_check CHECK (percent_off IS NULL OR (percent_off > 0 AND percent_off <= 100)),
    CONSTRAINT promo_codes_max_usages_check CHECK (max_usages IS NULL OR max_usages > 0),
    CONSTRAINT promo_codes_current_usages_check CHECK (current_usages >= 0),
    CONSTRAINT promo_codes_usage_limit_check CHECK (max_usages IS NULL OR current_usages <= max_usages),
    CONSTRAINT promo_codes_valid_window_check CHECK (valid_from IS NULL OR valid_until IS NULL OR valid_from <= valid_until),
    CONSTRAINT promo_codes_value_by_type_check CHECK (
        (type = 'duration_days' AND duration_days IS NOT NULL AND percent_off IS NULL) OR
        (type = 'percent_off' AND percent_off IS NOT NULL AND duration_days IS NULL)
    )
);

CREATE INDEX idx_promo_codes_is_active_valid_until ON promo_codes (is_active, valid_until);

CREATE TABLE promo_code_usages (
    id BIGSERIAL PRIMARY KEY,
    promo_code_id BIGINT NOT NULL REFERENCES promo_codes (id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT promo_code_usages_promo_code_id_user_id_unique UNIQUE (promo_code_id, user_id)
);

CREATE INDEX idx_promo_code_usages_promo_code_id ON promo_code_usages (promo_code_id);
CREATE INDEX idx_promo_code_usages_user_id ON promo_code_usages (user_id);
CREATE INDEX idx_promo_code_usages_user_id_applied_at ON promo_code_usages (user_id, applied_at DESC);

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_type TEXT NOT NULL,
    actor_id BIGINT,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id BIGINT,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT audit_logs_actor_type_check CHECK (actor_type IN ('system', 'user', 'admin', 'support')),
    CONSTRAINT audit_logs_entity_type_check CHECK (
        entity_type IN ('system', 'user', 'device', 'subscription', 'promo_code', 'promo_code_usage')
    )
);

CREATE INDEX idx_audit_logs_actor ON audit_logs (actor_type, actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_entity ON audit_logs (entity_type, entity_id, created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs (action, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE audit_logs;
DROP TABLE promo_code_usages;
DROP TABLE promo_codes;
-- +goose StatementEnd
