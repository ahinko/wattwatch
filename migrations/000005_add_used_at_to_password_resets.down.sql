-- Remove used_at column from password_resets table
ALTER TABLE password_resets DROP COLUMN IF EXISTS used_at;