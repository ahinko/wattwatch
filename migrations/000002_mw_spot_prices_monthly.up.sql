-- +migrate no-transaction

-- Create a continuous aggregate for monthly averages
CREATE MATERIALIZED VIEW spot_prices_monthly
    WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 month', timestamp) AS bucket,
    zone_id,
    currency_id,
    AVG(price) as avg_price,
    MIN(price) as min_price,
    MAX(price) as max_price,
    COUNT(*) as sample_count
FROM spot_prices
GROUP BY bucket, zone_id, currency_id;
