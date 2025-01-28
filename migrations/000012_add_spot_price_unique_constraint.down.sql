-- Remove unique constraint from spot_prices table
ALTER TABLE spot_prices DROP CONSTRAINT spot_prices_timestamp_zone_currency_unique; 