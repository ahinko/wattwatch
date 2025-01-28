-- Add verified_at column to email_verifications table
ALTER TABLE email_verifications ADD COLUMN verified_at TIMESTAMP WITH TIME ZONE; 