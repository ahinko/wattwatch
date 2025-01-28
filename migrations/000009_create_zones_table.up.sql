CREATE TABLE IF NOT EXISTS zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    timezone VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create an index on the name field for faster lookups
CREATE INDEX IF NOT EXISTS idx_zones_name ON zones(name);

-- Add audit trigger
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON zones
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- Insert default zones
INSERT INTO zones (name, timezone) VALUES
    ('SE1', 'Europe/Stockholm'),
    ('SE2', 'Europe/Stockholm'),
    ('SE3', 'Europe/Stockholm'),
    ('SE4', 'Europe/Stockholm')
ON CONFLICT (name) DO NOTHING; 