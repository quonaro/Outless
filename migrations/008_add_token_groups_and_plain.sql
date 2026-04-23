ALTER TABLE tokens
    ADD COLUMN IF NOT EXISTS token_plain TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tokens_token_plain
    ON tokens(token_plain)
    WHERE token_plain IS NOT NULL;

CREATE TABLE IF NOT EXISTS token_groups (
    token_id TEXT NOT NULL REFERENCES tokens(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (token_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_token_groups_token_id ON token_groups(token_id);
CREATE INDEX IF NOT EXISTS idx_token_groups_group_id ON token_groups(group_id);

INSERT INTO token_groups (token_id, group_id)
SELECT t.id, t.group_id
FROM tokens t
WHERE t.group_id IS NOT NULL
ON CONFLICT (token_id, group_id) DO NOTHING;
