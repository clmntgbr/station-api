-- +goose Up
-- +goose StatementBegin
ALTER TABLE fuel_prices_current RENAME TO current_prices;
ALTER TABLE fuel_prices_history RENAME TO prices_history;

ALTER INDEX fuel_prices_current_pkey RENAME TO current_prices_pkey;
ALTER INDEX fuel_prices_history_pkey RENAME TO prices_history_pkey;
ALTER TABLE current_prices RENAME CONSTRAINT fuel_prices_current_station_id_fkey TO current_prices_station_id_fkey;
ALTER TABLE prices_history RENAME CONSTRAINT fuel_prices_history_station_id_fkey TO prices_history_station_id_fkey;

ALTER INDEX idx_fuel_prices_current_fuel_price RENAME TO idx_current_prices_fuel_price;
ALTER INDEX idx_fuel_prices_history_station_fuel RENAME TO idx_prices_history_station_fuel;

ALTER SEQUENCE fuel_prices_history_id_seq RENAME TO prices_history_id_seq;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER SEQUENCE prices_history_id_seq RENAME TO fuel_prices_history_id_seq;

ALTER INDEX idx_prices_history_station_fuel RENAME TO idx_fuel_prices_history_station_fuel;
ALTER INDEX idx_current_prices_fuel_price RENAME TO idx_fuel_prices_current_fuel_price;

ALTER TABLE prices_history RENAME CONSTRAINT prices_history_station_id_fkey TO fuel_prices_history_station_id_fkey;
ALTER TABLE current_prices RENAME CONSTRAINT current_prices_station_id_fkey TO fuel_prices_current_station_id_fkey;
ALTER INDEX prices_history_pkey RENAME TO fuel_prices_history_pkey;
ALTER INDEX current_prices_pkey RENAME TO fuel_prices_current_pkey;

ALTER TABLE prices_history RENAME TO fuel_prices_history;
ALTER TABLE current_prices RENAME TO fuel_prices_current;
-- +goose StatementEnd
