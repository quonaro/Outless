ALTER TABLE nodes ADD COLUMN IF NOT EXISTS group_id TEXT;
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS group_id TEXT;

CREATE INDEX IF NOT EXISTS idx_nodes_group ON nodes(group_id);
CREATE INDEX IF NOT EXISTS idx_tokens_group ON tokens(group_id);

ALTER TABLE nodes ADD CONSTRAINT fk_nodes_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL;
ALTER TABLE tokens ADD CONSTRAINT fk_tokens_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL;
