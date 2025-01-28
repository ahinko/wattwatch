-- Remove verified_at column from email_verifications table
ALTER TABLE email_verifications DROP COLUMN IF EXISTS verified_at; 