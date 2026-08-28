-- Durable key/value state for the postgres build (TKT-VC27L3).
--
-- Backs internal/state.KV: the document render cache, user settings, the
-- operator logo/theme, and scheduler bookkeeping. On the filesystem build this
-- lives under the project's .rela/ directory, which is node-local — wrong for
-- the load-balanced multi-process deployment documented in
-- docs/postgres-backend.md, where an operator uploading a logo lands it on
-- whichever node served the POST while every other node keeps serving the old
-- one.
--
-- The key is the same `/`-separated hierarchical string the FS backend uses
-- (e.g. "documents/DOC-1-<hash>.html"), stored verbatim: no normalization, so
-- two keys differing only in case or separator stay distinct exactly as they do
-- on disk. COLLATE "C" makes comparison byte-exact for the same reason
-- entities.id uses it — a locale-sensitive collation could equate keys the
-- filesystem treats as different.
--
-- Values are BYTEA and read/written whole, matching `attachments`: the logo is
-- arbitrary uploaded bytes and a cached render is HTML, so neither is text and
-- neither streams. `state.KV` has no streaming API on any backend.
--
-- Deliberately NOT given a `seq` column: rela_seq feeds the change-feed
-- watermark (primeWatermark/catchUp scan entities/relations/deletions), and
-- burning sequence values on cache writes would erode the overlap budget and
-- drop real events. State is not part of the entity change feed.
CREATE TABLE state_kv (
    key        TEXT        COLLATE "C" PRIMARY KEY,
    value      BYTEA       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
