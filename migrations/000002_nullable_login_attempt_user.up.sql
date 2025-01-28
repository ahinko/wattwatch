-- Make user_id nullable in login_attempts table
ALTER TABLE login_attempts ALTER COLUMN user_id DROP NOT NULL; 