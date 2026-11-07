CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS urls (
    short_code VARCHAR(32) PRIMARY KEY,
    long_url TEXT NOT NULL,
    clicks INTEGER NOT NULL DEFAULT 0,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE, 
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_urls_user_id_created ON urls(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_urls_guest_expiry
    ON urls(expires_at) WHERE user_id IS NULL;
-- Volumes created before accounts or expiry existed need these applied once;
-- the init script only runs on an empty data directory.
ALTER TABLE urls ADD COLUMN IF NOT EXISTS user_id INTEGER REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE urls ALTER COLUMN short_code TYPE VARCHAR(32);
ALTER TABLE urls ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
