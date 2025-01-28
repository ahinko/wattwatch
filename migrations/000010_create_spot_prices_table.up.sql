CREATE TABLE IF NOT EXISTS spot_prices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    zone_id UUID NOT NULL REFERENCES zones(id),
    currency_id UUID NOT NULL REFERENCES currencies(id),
    price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create a unique index to prevent duplicates for (timestamp, zone_id, currency_id)
CREATE UNIQUE INDEX idx_spot_prices_unique ON spot_prices(timestamp, zone_id, currency_id);

-- Create indexes for common query patterns
CREATE INDEX idx_spot_prices_timestamp ON spot_prices(timestamp);
CREATE INDEX idx_spot_prices_zone_id ON spot_prices(zone_id);
CREATE INDEX idx_spot_prices_currency_id ON spot_prices(currency_id);

-- Add audit trigger for updated_at
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON spot_prices
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp(); 