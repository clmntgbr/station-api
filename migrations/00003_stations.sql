-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS stations (
    id TEXT PRIMARY KEY,
    content_hash TEXT NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS stations;
-- +goose StatementEnd
