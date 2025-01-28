-- +migrate no-transaction

-- Drop indexes first
DROP INDEX IF EXISTS idx_spot_prices_zone_currency_time;

-- Remove compression policy
SELECT remove_compression_policy('spot_prices', if_exists => TRUE);

-- Drop the hypertable
DROP TABLE IF EXISTS spot_prices;

-- Drop other tables in correct order (excluding spot_prices)
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS email_verifications;
DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS password_history;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS currencies;
DROP TABLE IF EXISTS zones;

-- Drop functions
DROP FUNCTION IF EXISTS trigger_set_timestamp;

-- Drop extensions (only if no other databases are using them)
DROP EXTENSION IF EXISTS "uuid-ossp";
DROP EXTENSION IF EXISTS timescaledb; 
