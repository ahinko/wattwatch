-- Add used_at column to password_resets table
ALTER TABLE password_resets ADD COLUMN used_at TIMESTAMP WITH TIME ZONE; 