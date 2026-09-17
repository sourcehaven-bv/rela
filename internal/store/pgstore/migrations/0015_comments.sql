-- pgstore schema, version 15: entity commentary (TKT-OGTVJW).
--
-- Backs internal/comments/pgcomments. Stage 1 (TKT-FIO205) shipped only
-- filecomments, which writes one YAML document per target under .rela/ and
-- says of itself that "a cross-process writer is out of scope for this tier,
-- exactly as it is for fsstore". docs/postgres-backend.md documents several
-- rela-server processes against ONE database, so on this build that tier is
-- wrong in a way an operator sees: a comment posted through one node is
-- invisible to every other node, with no error anywhere. Same defect and same
-- fix as state_kv (0008, TKT-VC27L3) — put the data in the database that is
-- already the source of truth.
--
-- Rows live in the tenant's schema like every other table here, so a
-- schema-per-tenant deployment scopes commentary for free.
--
-- # This table is NOT part of the graph
--
-- No foreign key to entities, and that is deliberate rather than an omission.
-- A comment is a remark ABOUT an entity, not a fact IN the operator's domain
-- model (see the internal/comments package doc), and the whole feature is
-- built outside store.Store, entitymanager, the audit log and /_schema. An FK
-- would drag commentary back into the graph's referential machinery: a cascade
-- delete would silently discard comments the service is supposed to remove
-- through its own lifecycle hooks (Service.EntityDeleted), and a rename —
-- which rela performs as an in-place re-key — would have to be taught about a
-- table the store knows nothing about. The service owns this lifecycle; see
-- pgcomments.Store.Rename and DeleteAllFaces.
--
-- It also means a comment can outlive its entity. That is the SAME property
-- the file backend has (a stray .rela/comments/TKT-1.yaml after a manual
-- entity deletion) and it is survivable: an unreachable row is invisible,
-- because every read is by target key.
--
-- # Identity and ordering
--
-- target_key is entity.FormatStateRef(id, face) — the bare id for the DEFAULT
-- face, "id@face" otherwise — which is exactly the key filecomments uses for
-- its filename. Storing the composed key rather than (id, face) columns keeps
-- one definition of "which thread is this" across all four backends, and it is
-- what lets faces exist without a migration: every pre-faces thread is already
-- at its correct key.
--
-- COLLATE "C" for the same reason state_kv.key and entities.id take it: the
-- key is matched byte-exactly, and a locale-sensitive collation could equate
-- two keys the other backends treat as distinct.
--
-- (created_at, id) is the contract's sort order, pinned by
-- commentstest.RunOrderingTests: oldest first, the server-minted id breaking
-- ties so a coarse clock stamping two comments in one tick cannot make a
-- thread reorder between reads. The index carries both columns so it CAN serve
-- List pre-ordered.
--
-- It usually will not, and that is fine. Measured on 200k rows, the planner
-- prefers the narrower comments_target_prefix_idx and sorts, because a thread
-- is capped at MaxPerTarget and sorting a few hundred rows costs less than the
-- wider index's I/O. It does choose this index, with no Sort node, once a
-- single thread grows large. Treat it as insurance for the threads that would
-- hurt, not as the plan every List takes — and re-measure before concluding a
-- sort in the plan is a regression.
--
-- # Why the anchor is jsonb
--
-- comments.Anchor is a discriminated union: two name-based kinds (property,
-- section) carry only a ref, while a text anchor carries a six-field
-- descriptor set used to RE-LOCATE a quote after the body is edited. Columns
-- would mean six mostly-NULL ones plus a migration every time a kind is added,
-- and the type's doc explicitly requires that "adding a kind must not require
-- migrating stored comments". Nothing queries inside the anchor — it is read
-- whole, handed to the resolver, and written whole — so it needs no GIN index.

CREATE TABLE comments (
    -- Server-minted (never client-supplied — see the package doc on why a
    -- caller-chosen id would let one principal overwrite another's comment).
    id          TEXT        COLLATE "C" NOT NULL,
    target_key  TEXT        COLLATE "C" NOT NULL,
    -- The entity type the thread hangs off. NOT part of the key and NOT read
    -- back by this backend: entity ids are unique across types, so target_key
    -- alone identifies a thread and every query keys on it.
    --
    -- Stored anyway because a comment row is otherwise unintelligible to
    -- anyone reading the table directly — an operator answering "what is
    -- commented on" from SQL, or triaging a support question, would otherwise
    -- have to join back through the entity store to learn what a key refers
    -- to. It costs one column on a table that will never be large.
    target_type TEXT        NOT NULL,
    -- Written from the request principal, never from the request body. This is
    -- what makes authorship unforgeable, and it is why the column is NOT NULL
    -- with no default: comments.ErrUnknownAuthor refuses the write instead, so
    -- an unattributable comment is never stored (an "unknown" author would
    -- make every *-own permission check meaningless).
    author      TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    -- Zero until the first edit. NULL rather than a sentinel timestamp so
    -- "never edited" is representable without picking a magic instant;
    -- comments.Comment.UpdatedAt is omitzero on the wire for the same reason.
    updated_at  TIMESTAMPTZ,
    anchor      JSONB       NOT NULL,
    body        TEXT        NOT NULL,
    resolved    BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Per-target rather than global: ids are minted per comment and the
    -- service addresses one by (target, id), so this is the uniqueness that
    -- actually holds. It also makes Update/Delete's "not found" a plain
    -- zero-rows-affected check against the same key the caller supplied.
    PRIMARY KEY (target_key, id)
);

-- Serves List (the only read): one target's thread in contract order.
CREATE INDEX comments_thread_idx ON comments (target_key, created_at, id);

-- Serves Rename and DeleteAllFaces, which must reach EVERY face of an entity —
-- the bare id plus any "id@face" — from the id alone. Without this, both
-- degrade to a scan of the whole table, and a rename is on the entity write
-- path. text_pattern_ops because the query is a prefix LIKE, which the default
-- collation's index cannot serve (the same reason entities_id_prefix_idx takes
-- it in 0014).
CREATE INDEX comments_target_prefix_idx ON comments (target_key text_pattern_ops);
