-- Add missing columns to audit_logs table
ALTER TABLE audit_logs
    ADD COLUMN entity_type VARCHAR(50),
    ADD COLUMN entity_id VARCHAR(255),
    ADD COLUMN description TEXT,
    ADD COLUMN metadata TEXT; 