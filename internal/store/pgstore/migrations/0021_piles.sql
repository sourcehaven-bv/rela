-- pgstore schema, version 20: per-user piles (TKT-K3RJLH).
--
-- Backs internal/piles/pgpiles. A pile is a named, personal stack of entity
-- references. The single-process tiers keep piles in one state.KV document,
-- rewritten whole on every change; several rela-server processes against one
-- database would lose each other's writes that way, so this build keeps one
-- row per pile and one row per item.
--
-- Rows live in the tenant's schema like every other table here, so a
-- schema-per-tenant deployment scopes piles for free.
--
-- # Not graph content
--
-- No foreign key to entities. A pile is a fact about one person's work, not
-- about the entities on it (see the internal/piles package doc), and the
-- piles service owns its own rename and delete lifecycle through the
-- entitymanager alias hook. An item may outlive its entity briefly; every read
-- path resolves items through the reader's visibility gate, so a dangling row
-- is invisible.
--
-- Ids are matched byte-exactly, hence COLLATE "C" as on entities.id.

CREATE TABLE piles (
    -- Server-minted (PIL-...), never client-supplied.
    id         TEXT        COLLATE "C" PRIMARY KEY,
    -- principal.User: a person entity id with person mapping, a login name
    -- without. Never empty, never a system identity (the service refuses both).
    owner      TEXT        COLLATE "C" NOT NULL,
    name       TEXT        NOT NULL,
    icon       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Names are unique per owner, compared case-insensitively. This index is what
-- makes a duplicate name an atomic refusal rather than a check-then-insert race.
CREATE UNIQUE INDEX piles_owner_name_idx ON piles (owner, lower(name));

-- Serves ListPiles and the owner rewrite of a renamed or deleted person.
CREATE INDEX piles_owner_idx ON piles (owner);

-- A dedicated sequence orders items. Never rela_seq: that one feeds the change
-- feed's watermark, and burning it on rows outside entities and relations would
-- erode the catch-up overlap budget.
CREATE SEQUENCE pile_item_seq;

CREATE TABLE pile_items (
    pile_id   TEXT        COLLATE "C" NOT NULL REFERENCES piles (id) ON DELETE CASCADE,
    entity_id TEXT        COLLATE "C" NOT NULL,
    -- '' is the implicit face of a faceless type.
    face      TEXT        COLLATE "C" NOT NULL DEFAULT '',
    -- Newest item = highest seq. Eviction keeps the highest seqs.
    seq       BIGINT      NOT NULL DEFAULT nextval('pile_item_seq'),
    added_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (pile_id, entity_id, face)
);

-- Serves the rename and delete hooks, which reach every pile by entity id.
CREATE INDEX pile_items_entity_idx ON pile_items (entity_id);

-- Serves reads in stack order and the eviction's top-N scan.
CREATE INDEX pile_items_order_idx ON pile_items (pile_id, seq DESC);
