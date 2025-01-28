-- Remove indices from users table
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_role_id;
DROP INDEX IF EXISTS idx_users_created_at;

-- Remove indices from email verifications
DROP INDEX IF EXISTS idx_email_verifications_user_id;
DROP INDEX IF EXISTS idx_email_verifications_token;

-- Remove indices from password resets
DROP INDEX IF EXISTS idx_password_resets_user_id;
DROP INDEX IF EXISTS idx_password_resets_token;

-- Remove indices from refresh tokens
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
DROP INDEX IF EXISTS idx_refresh_tokens_token;
DROP INDEX IF EXISTS idx_refresh_tokens_expires_at;

-- Remove indices from roles
DROP INDEX IF EXISTS idx_roles_name; 