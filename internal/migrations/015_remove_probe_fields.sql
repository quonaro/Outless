-- Remove auto-delete-unavailable, latency, status fields from groups and nodes tables
-- This migration removes probe-related functionality

-- Remove auto_delete_unavailable from groups table
ALTER TABLE groups DROP COLUMN IF EXISTS auto_delete_unavailable;

-- Remove healthy_nodes, unhealthy_nodes, unknown_nodes from groups table
ALTER TABLE groups DROP COLUMN IF EXISTS healthy_nodes;
ALTER TABLE groups DROP COLUMN IF EXISTS unhealthy_nodes;
ALTER TABLE groups DROP COLUMN IF EXISTS unknown_nodes;

-- Remove latency_ms from nodes table
ALTER TABLE nodes DROP COLUMN IF EXISTS latency_ms;

-- Remove status from nodes table
ALTER TABLE nodes DROP COLUMN IF EXISTS status;

-- Remove last_checked_at from nodes table (probe-related)
ALTER TABLE nodes DROP COLUMN IF EXISTS last_checked_at;
