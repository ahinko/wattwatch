-- Remove added columns from audit_logs table
ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS entity_type,
    DROP COLUMN IF EXISTS entity_id,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS metadata; 