-- +migrate no-transaction

-- Create a continuous aggregate for hourly averages
CREATE MATERIALIZED VIEW spot_prices_hourly
    WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', timestamp) AS bucket,
    zone_id,
    currency_id,
    AVG(price) as avg_price,
    MIN(price) as min_price,
    MAX(price) as max_price,
    COUNT(*) as sample_count
FROM spot_prices
GROUP BY bucket, zone_id, currency_id;

-- Create a continuous aggregate for daily averages
CREATE MATERIALIZED VIEW spot_prices_daily
    WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', timestamp) AS bucket,
    zone_id,
    currency_id,
    AVG(price) as avg_price,
    MIN(price) as min_price,
    MAX(price) as max_price,
    COUNT(*) as sample_count
FROM spot_prices
GROUP BY bucket, zone_id, currency_id;

-- Add refresh policies for continuous aggregates
SELECT add_continuous_aggregate_policy('spot_prices_hourly',
    start_offset => INTERVAL '1 month',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE
);

SELECT add_continuous_aggregate_policy('spot_prices_daily',
    start_offset => INTERVAL '1 year',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
); 