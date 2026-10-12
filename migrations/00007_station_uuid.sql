-- +goose Up
-- +goose StatementBegin
-- PostgreSQL 17 has no uuidv7(). New rows are generated in Go; this function only backfills existing stations.
CREATE FUNCTION carbu_uuid_v7() RETURNS uuid
LANGUAGE plpgsql VOLATILE AS $$
DECLARE
    unix_ts_ms bytea;
    uuid_bytes bytea;
BEGIN
    unix_ts_ms = substring(int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3);
    uuid_bytes = uuid_send(gen_random_uuid());
    uuid_bytes = overlay(uuid_bytes placing unix_ts_ms FROM 1 FOR 6);
    uuid_bytes = set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 15) | 112);
    uuid_bytes = set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 63) | 128);
    RETURN encode(uuid_bytes, 'hex')::uuid;
END
$$;

ALTER TABLE stations ADD COLUMN station_id TEXT;
UPDATE stations SET station_id = id;
ALTER TABLE stations ALTER COLUMN station_id SET NOT NULL;

ALTER TABLE stations ADD COLUMN id_uuid UUID;
UPDATE stations SET id_uuid = carbu_uuid_v7();
ALTER TABLE stations ALTER COLUMN id_uuid SET NOT NULL;

ALTER TABLE current_prices DROP CONSTRAINT current_prices_station_id_fkey;
ALTER TABLE prices_history DROP CONSTRAINT prices_history_station_id_fkey;

ALTER TABLE current_prices DROP CONSTRAINT current_prices_pkey;
ALTER TABLE current_prices ADD COLUMN station_uuid UUID;
UPDATE current_prices AS prices
SET station_uuid = stations.id_uuid
FROM stations
WHERE stations.id = prices.station_id;
ALTER TABLE current_prices ALTER COLUMN station_uuid SET NOT NULL;
ALTER TABLE current_prices DROP COLUMN station_id;
ALTER TABLE current_prices RENAME COLUMN station_uuid TO station_id;
ALTER TABLE current_prices ADD CONSTRAINT current_prices_pkey PRIMARY KEY (station_id, fuel);

ALTER TABLE prices_history ADD COLUMN station_uuid UUID;
UPDATE prices_history AS prices
SET station_uuid = stations.id_uuid
FROM stations
WHERE stations.id = prices.station_id;
ALTER TABLE prices_history ALTER COLUMN station_uuid SET NOT NULL;
ALTER TABLE prices_history DROP COLUMN station_id;
ALTER TABLE prices_history RENAME COLUMN station_uuid TO station_id;
CREATE INDEX idx_prices_history_station_fuel ON prices_history (station_id, fuel, observed_at);

ALTER TABLE stations DROP CONSTRAINT stations_pkey;
ALTER TABLE stations DROP COLUMN id;
ALTER TABLE stations RENAME COLUMN id_uuid TO id;
ALTER TABLE stations ADD CONSTRAINT stations_pkey PRIMARY KEY (id);
ALTER TABLE stations ADD CONSTRAINT stations_station_id_key UNIQUE (station_id);

ALTER TABLE current_prices
    ADD CONSTRAINT current_prices_station_id_fkey
    FOREIGN KEY (station_id) REFERENCES stations (id);
ALTER TABLE prices_history
    ADD CONSTRAINT prices_history_station_id_fkey
    FOREIGN KEY (station_id) REFERENCES stations (id);

DROP FUNCTION carbu_uuid_v7();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE current_prices DROP CONSTRAINT current_prices_station_id_fkey;
ALTER TABLE prices_history DROP CONSTRAINT prices_history_station_id_fkey;

ALTER TABLE current_prices ADD COLUMN feed_id TEXT;
UPDATE current_prices AS prices
SET feed_id = stations.station_id
FROM stations
WHERE stations.id = prices.station_id;
ALTER TABLE current_prices DROP CONSTRAINT current_prices_pkey;
ALTER TABLE current_prices DROP COLUMN station_id;
ALTER TABLE current_prices RENAME COLUMN feed_id TO station_id;
ALTER TABLE current_prices ALTER COLUMN station_id SET NOT NULL;
ALTER TABLE current_prices ADD CONSTRAINT current_prices_pkey PRIMARY KEY (station_id, fuel);

ALTER TABLE prices_history ADD COLUMN feed_id TEXT;
UPDATE prices_history AS prices
SET feed_id = stations.station_id
FROM stations
WHERE stations.id = prices.station_id;
ALTER TABLE prices_history DROP COLUMN station_id;
ALTER TABLE prices_history RENAME COLUMN feed_id TO station_id;
ALTER TABLE prices_history ALTER COLUMN station_id SET NOT NULL;
CREATE INDEX idx_prices_history_station_fuel ON prices_history (station_id, fuel, observed_at);

ALTER TABLE stations DROP CONSTRAINT stations_station_id_key;
ALTER TABLE stations DROP CONSTRAINT stations_pkey;
ALTER TABLE stations DROP COLUMN id;
ALTER TABLE stations RENAME COLUMN station_id TO id;
ALTER TABLE stations ADD CONSTRAINT stations_pkey PRIMARY KEY (id);

ALTER TABLE current_prices
    ADD CONSTRAINT current_prices_station_id_fkey
    FOREIGN KEY (station_id) REFERENCES stations (id);
ALTER TABLE prices_history
    ADD CONSTRAINT prices_history_station_id_fkey
    FOREIGN KEY (station_id) REFERENCES stations (id);
-- +goose StatementEnd
