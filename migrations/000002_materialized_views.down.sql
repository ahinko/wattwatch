-- +migrate no-transaction

-- Drop continuous aggregate policies first
SELECT remove_continuous_aggregate_policy('spot_prices_hourly', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('spot_prices_daily', if_exists => TRUE);

-- Drop the continuous aggregates
DROP MATERIALIZED VIEW IF EXISTS spot_prices_hourly;
DROP MATERIALIZED VIEW IF EXISTS spot_prices_daily;

