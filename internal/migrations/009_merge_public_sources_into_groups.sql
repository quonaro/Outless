ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS source_url TEXT,
    ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ;

WITH ranked_sources AS (
    SELECT
        ps.group_id,
        ps.url,
        ps.last_fetched_at,
        ROW_NUMBER() OVER (PARTITION BY ps.group_id ORDER BY ps.created_at ASC) AS rn
    FROM public_sources ps
)
UPDATE groups g
SET source_url = rs.url,
    last_synced_at = rs.last_fetched_at
FROM ranked_sources rs
WHERE rs.rn = 1
  AND rs.group_id = g.id
  AND (g.source_url IS NULL OR g.source_url = '');
