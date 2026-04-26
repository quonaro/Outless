-- Drop indexes on removed probe-related columns
-- These indexes were created before probe fields were removed

DROP INDEX IF EXISTS idx_nodes_status_latency_created;
DROP INDEX IF EXISTS idx_nodes_status;
