CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'unknown',
    country TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tokens (
    id TEXT PRIMARY KEY,
    owner TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tokens_active_expires
    ON tokens(is_active, expires_at);
