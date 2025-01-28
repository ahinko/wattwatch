CREATE TABLE IF NOT EXISTS currencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(3) NOT NULL UNIQUE CHECK (LENGTH(name) = 3),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create an index on the name field for faster lookups
CREATE INDEX IF NOT EXISTS idx_currencies_name ON currencies(name);

-- Add audit trigger
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON currencies
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- Insert default currencies
INSERT INTO currencies (name) VALUES
    ('EUR'),
    ('SEK')
ON CONFLICT (name) DO NOTHING; 