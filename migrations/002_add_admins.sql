CREATE TABLE IF NOT EXISTS admins (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Default admin user (password: admin123)
-- Hash generated with bcrypt cost 10
INSERT INTO admins (id, username, password_hash, created_at)
VALUES (
    'admin_default',
    'admin',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    NOW()
) ON CONFLICT (username) DO NOTHING;
