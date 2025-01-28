-- Add unique constraint on spot_prices table
ALTER TABLE spot_prices ADD CONSTRAINT spot_prices_timestamp_zone_currency_unique UNIQUE (timestamp, zone_id, currency_id); 