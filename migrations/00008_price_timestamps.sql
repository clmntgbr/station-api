-- +goose Up
-- +goose StatementBegin
ALTER TABLE current_prices
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE current_prices
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS created_at;
-- +goose StatementEnd
