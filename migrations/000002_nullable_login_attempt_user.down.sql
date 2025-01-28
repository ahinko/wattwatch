-- Make user_id NOT NULL in login_attempts table
DELETE FROM login_attempts WHERE user_id IS NULL;
ALTER TABLE login_attempts ALTER COLUMN user_id SET NOT NULL; 