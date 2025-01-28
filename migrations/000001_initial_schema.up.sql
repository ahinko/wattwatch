-- +migrate no-transaction

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create roles table with UUID and soft delete
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL UNIQUE,
    is_protected BOOLEAN NOT NULL DEFAULT false,
    is_admin_group BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create updated_at trigger for roles
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON roles
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- Insert default roles
INSERT INTO roles (name, is_protected, is_admin_group) VALUES
    ('admin', true, true),
    ('user', true, false);

-- Create users table with UUID and soft delete
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE,
    role_id UUID NOT NULL REFERENCES roles(id),
    email_verified BOOLEAN NOT NULL DEFAULT false,
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    last_failed_login TIMESTAMP WITH TIME ZONE,
    last_login_at TIMESTAMP WITH TIME ZONE,
    password_changed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create updated_at trigger for users
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- Create indexes for users table
CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_username ON users(username) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_role_id ON users(role_id) WHERE deleted_at IS NULL;

-- Create password_history table with UUID references
CREATE TABLE password_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for password_history
CREATE INDEX idx_password_history_user_id ON password_history(user_id);
CREATE INDEX idx_password_history_created_at ON password_history(created_at);

-- Create login_attempts table with UUID references
CREATE TABLE login_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    ip VARCHAR(45) NOT NULL,
    success BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for login_attempts
CREATE INDEX idx_login_attempts_user_id ON login_attempts(user_id);
CREATE INDEX idx_login_attempts_ip ON login_attempts(ip);
CREATE INDEX idx_login_attempts_created_at ON login_attempts(created_at);
CREATE INDEX idx_login_attempts_user_ip ON login_attempts(user_id, ip, created_at);

-- Create email_verifications table with UUID references
CREATE TABLE email_verifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    verified_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for email_verifications
CREATE INDEX idx_email_verifications_user_id ON email_verifications(user_id);
CREATE INDEX idx_email_verifications_token ON email_verifications(token);
CREATE INDEX idx_email_verifications_expires_at ON email_verifications(expires_at);

-- Create password_resets table with UUID references
CREATE TABLE password_resets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for password_resets
CREATE INDEX idx_password_resets_user_id ON password_resets(user_id);
CREATE INDEX idx_password_resets_token ON password_resets(token);
CREATE INDEX idx_password_resets_expires_at ON password_resets(expires_at);

-- Create refresh_tokens table
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for refresh_tokens
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

-- Create audit_logs table with UUID references
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    action VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50),
    entity_id UUID,
    description TEXT,
    metadata JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for audit_logs
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_description_gin ON audit_logs USING gin(to_tsvector('english', description));
CREATE INDEX idx_audit_logs_metadata ON audit_logs USING gin(metadata);

-- Create currencies table
CREATE TABLE currencies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(3) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create updated_at trigger for currencies
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON currencies
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- Create indexes for currencies
CREATE INDEX idx_currencies_name ON currencies(name) WHERE deleted_at IS NULL;

-- Insert default currencies
INSERT INTO currencies (name) VALUES
    ('EUR'),
    ('SEK')
ON CONFLICT (name) DO NOTHING; 

-- Create zones table
CREATE TABLE zones (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL UNIQUE,
    timezone VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create updated_at trigger for zones
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON zones
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- Create indexes for zones
CREATE INDEX idx_zones_name ON zones(name) WHERE deleted_at IS NULL;

-- Insert default zones
INSERT INTO zones (name, timezone) VALUES
    ('SE1', 'Europe/Stockholm'),
    ('SE2', 'Europe/Stockholm'),
    ('SE3', 'Europe/Stockholm'),
    ('SE4', 'Europe/Stockholm')
ON CONFLICT (name) DO NOTHING; 

-- Create spot_prices table optimized for time-series data
CREATE TABLE spot_prices (
    id UUID DEFAULT uuid_generate_v4(),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    zone_id UUID NOT NULL REFERENCES zones(id),
    currency_id UUID NOT NULL REFERENCES currencies(id),
    price DECIMAL(10,4) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(timestamp, zone_id, currency_id)
);

-- Create indexes for common queries
CREATE INDEX idx_spot_prices_zone_currency_time 
    ON spot_prices (zone_id, currency_id, timestamp DESC);

-- Convert spot_prices to hypertable
SELECT create_hypertable('spot_prices', 'timestamp', 
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Add compression policy (compress chunks older than 7 days)
ALTER TABLE spot_prices SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'zone_id,currency_id'
);

SELECT add_compression_policy('spot_prices', 
    compress_after => INTERVAL '7 days',
    if_not_exists => TRUE
);