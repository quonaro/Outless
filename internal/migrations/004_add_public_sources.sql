CREATE TABLE IF NOT EXISTS public_sources (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    group_id TEXT NOT NULL,
    last_fetched_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_public_sources_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_public_sources_group ON public_sources(group_id);
