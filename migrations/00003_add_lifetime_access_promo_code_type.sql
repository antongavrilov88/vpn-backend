-- +goose Up
-- +goose StatementBegin
ALTER TABLE promo_codes
    DROP CONSTRAINT promo_codes_type_check,
    DROP CONSTRAINT promo_codes_value_by_type_check;

ALTER TABLE promo_codes
    ADD CONSTRAINT promo_codes_type_check CHECK (type IN ('duration_days', 'percent_off', 'lifetime_access')),
    ADD CONSTRAINT promo_codes_value_by_type_check CHECK (
        (type = 'duration_days' AND duration_days IS NOT NULL AND percent_off IS NULL) OR
        (type = 'percent_off' AND percent_off IS NOT NULL AND duration_days IS NULL) OR
        (type = 'lifetime_access' AND duration_days IS NULL AND percent_off IS NULL)
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE promo_codes
    DROP CONSTRAINT promo_codes_type_check,
    DROP CONSTRAINT promo_codes_value_by_type_check;

ALTER TABLE promo_codes
    ADD CONSTRAINT promo_codes_type_check CHECK (type IN ('duration_days', 'percent_off')),
    ADD CONSTRAINT promo_codes_value_by_type_check CHECK (
        (type = 'duration_days' AND duration_days IS NOT NULL AND percent_off IS NULL) OR
        (type = 'percent_off' AND percent_off IS NOT NULL AND duration_days IS NULL)
    );
-- +goose StatementEnd
