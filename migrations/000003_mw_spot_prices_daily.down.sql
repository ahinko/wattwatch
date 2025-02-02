-- +migrate no-transaction

-- Drop the continuous aggregates
DROP MATERIALIZED VIEW IF EXISTS spot_prices_daily;

