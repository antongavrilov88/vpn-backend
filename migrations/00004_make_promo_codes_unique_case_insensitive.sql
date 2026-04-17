-- +goose Up
-- +goose StatementBegin
ALTER TABLE promo_codes
    DROP CONSTRAINT promo_codes_code_unique;

CREATE UNIQUE INDEX promo_codes_code_lower_unique_idx ON promo_codes (LOWER(code));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS promo_codes_code_lower_unique_idx;

ALTER TABLE promo_codes
    ADD CONSTRAINT promo_codes_code_unique UNIQUE (code);
-- +goose StatementEnd
