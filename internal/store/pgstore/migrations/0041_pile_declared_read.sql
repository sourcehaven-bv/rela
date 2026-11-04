-- pgstore schema, version 41: declared piles and read state on pile items.
--
-- A declared pile is one the operator names in schema.yaml (`piles:`). Every
-- user has it, it is found by its key, and it takes no part in the name rule
-- of the piles a user makes. Its items may carry who caused them, a reason
-- key, and when the owner read them.

-- '' for a pile the user made, the schema key for a declared pile.
ALTER TABLE piles ADD COLUMN IF NOT EXISTS key TEXT COLLATE "C" NOT NULL DEFAULT '';

-- Names stay unique among the piles a user makes; declared piles are keyed.
DROP INDEX IF EXISTS piles_owner_name_idx;
CREATE UNIQUE INDEX piles_owner_name_idx ON piles (owner, lower(name)) WHERE key = '';
CREATE UNIQUE INDEX IF NOT EXISTS piles_owner_key_idx ON piles (owner, key) WHERE key <> '';

-- The user whose write put the item there, '' when unknown.
ALTER TABLE pile_items ADD COLUMN IF NOT EXISTS actor TEXT COLLATE "C" NOT NULL DEFAULT '';
-- A key of the declared pile's reasons, '' for none.
ALTER TABLE pile_items ADD COLUMN IF NOT EXISTS reason TEXT NOT NULL DEFAULT '';
-- When the owner read the item; NULL while unread.
ALTER TABLE pile_items ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ;
