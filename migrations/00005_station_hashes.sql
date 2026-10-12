-- +goose Up
-- +goose StatementBegin
ALTER TABLE stations
    ADD COLUMN info_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN price_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE stations
    DROP COLUMN content_hash;

ALTER TABLE stations
    ALTER COLUMN info_hash DROP DEFAULT,
    ALTER COLUMN price_hash DROP DEFAULT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE stations
    ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE stations
    DROP COLUMN info_hash,
    DROP COLUMN price_hash;

ALTER TABLE stations
    ALTER COLUMN content_hash DROP DEFAULT;
-- +goose StatementEnd
