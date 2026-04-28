-- Make group_id mandatory for nodes
-- First, set NULL group_id to a default or delete nodes without groups
-- For now, we'll delete nodes without groups as they shouldn't exist
DELETE FROM nodes WHERE group_id IS NULL;

-- Make group_id NOT NULL
ALTER TABLE nodes ALTER COLUMN group_id SET NOT NULL;
