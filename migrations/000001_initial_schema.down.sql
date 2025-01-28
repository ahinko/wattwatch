-- Drop tables in reverse order
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS email_verifications;
DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS password_history;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;

-- Drop trigger function
DROP FUNCTION IF EXISTS trigger_set_timestamp();

-- Drop UUID extension
DROP EXTENSION IF EXISTS "uuid-ossp"; 