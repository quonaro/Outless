ALTER TABLE tokens ADD COLUMN IF NOT EXISTS uuid TEXT;

UPDATE tokens SET uuid = gen_random_uuid()::text WHERE uuid IS NULL;

ALTER TABLE tokens ALTER COLUMN uuid SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tokens_uuid ON tokens(uuid);
