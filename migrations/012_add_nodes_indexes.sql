-- Add composite index for nodes sorting optimization
-- Optimizes query: SELECT * FROM nodes ORDER BY status, latency_ms, created_at
CREATE INDEX IF NOT EXISTS idx_nodes_status_latency_created
    ON nodes(status, latency_ms, created_at DESC);
