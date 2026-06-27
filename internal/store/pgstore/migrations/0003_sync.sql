-- pgstore schema, version 3: deletion tombstones + seq indexes for sync.
--
-- The sync feature (FEAT-NJ9FEN) needs to answer "what changed since cursor X",
-- including deletions. Deletes are HARD deletes, so a `seq > X` scan of the live
-- entities/relations tables can never report a removed row. The `deletions`
-- tombstone table records each delete with its own seq from the shared rela_seq
-- sequence, so the manifest is "live rows with seq > X" UNION "tombstones with
-- seq > X". A tombstone is written in the SAME transaction as the delete, so the
-- record of the removal is atomic with the removal itself.
--
-- Soft-delete (a deleted_at flag on the live row) was rejected in planning: it
-- would force `deleted_at IS NULL` filters across every read/list/search/cascade
-- path. A separate tombstone table keeps all existing reads untouched.

CREATE TABLE deletions (
    -- kind is 'e' (entity) or 'r' (relation), matching the catch-up/feed
    -- discriminator. id_a/id_b/id_c hold the deleted record's key: for an
    -- entity, id_a = id (b, c empty); for a relation, id_a/id_b/id_c =
    -- from_id/rel_type/to_id. typ carries the entity type so a tombstone can
    -- emit a structurally complete EventEntityDeleted (relations carry no type).
    kind       TEXT        COLLATE "C" NOT NULL,
    id_a       TEXT        COLLATE "C" NOT NULL,
    id_b       TEXT        COLLATE "C" NOT NULL DEFAULT '',
    id_c       TEXT        COLLATE "C" NOT NULL DEFAULT '',
    typ        TEXT        NOT NULL DEFAULT '',
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- seq from the shared sequence so a tombstone orders against live-row seqs
    -- in a single manifest cursor. No primary key: the same key can be deleted,
    -- recreated, and deleted again, producing multiple tombstones over time —
    -- each is a distinct event with its own seq.
    seq        BIGINT      NOT NULL DEFAULT nextval('rela_seq')
);

-- Manifest/catch-up scans are "WHERE seq > $1 ORDER BY seq" over each table.
-- Without these indexes that is a seqscan + sort on every poll; with them it is
-- an index range scan. entities/relations were unindexed on seq until now.
--
-- LOCK NOTE: these CREATE INDEX statements are NOT CONCURRENT — pgstore.Migrate
-- runs every migration in ONE transaction under an advisory lock, and CREATE
-- INDEX CONCURRENTLY cannot run inside a transaction block. On an already-large
-- entities/relations table this takes a SHARE lock that blocks writes for the
-- duration of the build. This is the first migration to index a table that may
-- hold production data, so a deployment that upgrades over a large dataset
-- should expect a brief write stall (apply during a maintenance window). A
-- concurrent-index migration path would require restructuring the runner.
CREATE INDEX entities_seq_idx  ON entities  (seq);
CREATE INDEX relations_seq_idx ON relations (seq);
CREATE INDEX deletions_seq_idx ON deletions (seq);
