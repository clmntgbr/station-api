-- +goose Up
-- +goose StatementBegin
-- WGS84 degrees. A geography column needs PostGIS, which this database image does not provide.
ALTER TABLE stations
    ADD COLUMN energy_type TEXT NOT NULL,
    ADD COLUMN latitude DOUBLE PRECISION NOT NULL,
    ADD COLUMN longitude DOUBLE PRECISION NOT NULL,
    ADD COLUMN postal_code TEXT NOT NULL,
    ADD COLUMN address TEXT NOT NULL,
    ADD COLUMN city TEXT NOT NULL,
    ADD COLUMN pop TEXT NOT NULL,
    ADD COLUMN automate_24 BOOLEAN NOT NULL,
    ADD COLUMN hours JSONB NOT NULL,
    ADD COLUMN services JSONB NOT NULL,
    ADD COLUMN google_place_id TEXT NULL;

CREATE TABLE fuel_prices_current (
    station_id TEXT NOT NULL REFERENCES stations (id),
    fuel TEXT NOT NULL,
    price NUMERIC(6, 3) NOT NULL,
    source_updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (station_id, fuel)
);

CREATE INDEX idx_fuel_prices_current_fuel_price ON fuel_prices_current (fuel, price);

CREATE TABLE fuel_prices_history (
    id BIGSERIAL PRIMARY KEY,
    station_id TEXT NOT NULL REFERENCES stations (id),
    fuel TEXT NOT NULL,
    price NUMERIC(6, 3) NOT NULL,
    source_updated_at TIMESTAMPTZ NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_fuel_prices_history_station_fuel ON fuel_prices_history (station_id, fuel, observed_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS fuel_prices_history;
DROP TABLE IF EXISTS fuel_prices_current;

ALTER TABLE stations
    DROP COLUMN IF EXISTS google_place_id,
    DROP COLUMN IF EXISTS services,
    DROP COLUMN IF EXISTS hours,
    DROP COLUMN IF EXISTS automate_24,
    DROP COLUMN IF EXISTS pop,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS postal_code,
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS latitude,
    DROP COLUMN IF EXISTS energy_type;
-- +goose StatementEnd
