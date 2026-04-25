-- Add random selection and limit parameters to groups for subscription optimization
-- This allows limiting the number of nodes returned per group and optionally randomizing selection

ALTER TABLE groups ADD COLUMN IF NOT EXISTS random_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS random_limit INTEGER DEFAULT NULL;
-- Add check constraint to ensure random_limit is positive if set
ALTER TABLE groups ADD CONSTRAINT chk_groups_random_limit_positive CHECK (random_limit IS NULL OR random_limit > 0);
