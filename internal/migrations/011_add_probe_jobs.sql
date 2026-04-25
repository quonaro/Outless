CREATE TABLE IF NOT EXISTS probe_jobs (
    id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL DEFAULT '',
    node_id TEXT NOT NULL,
    group_id TEXT,
    requested_by TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL DEFAULT 'normal',
    probe_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_probe_jobs_status_created_at
    ON probe_jobs(status, created_at);

CREATE INDEX IF NOT EXISTS idx_probe_jobs_group_status
    ON probe_jobs(group_id, status);

CREATE INDEX IF NOT EXISTS idx_probe_jobs_batch_id
    ON probe_jobs(batch_id);
