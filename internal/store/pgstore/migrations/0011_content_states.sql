-- pgstore schema, version 11: content states (TKT-DOFYR1, FEAT-9CD2MX).
--
-- An entity may hold several content STATES addressed by (id, pointer);
-- the empty pointer is the default state, so every existing row already
-- is its own default state and this migration rewrites no data. The
-- pointer value is the canonical serialized coordinate produced by the
-- entity.Pointer codec; the store only ever equality-matches it, which
-- is why one COLLATE "C" text column suffices (and stays sufficient when
-- multi-axis coordinates arrive — the resolver compiles worlds to sets
-- of concrete coordinates, the column never changes shape).
--
-- '' NOT NULL rather than NULL: the pointer joins both primary keys —
-- the same (from,type,to) triple from two different states is two
-- distinct edges — and PK columns cannot be NULL. One convention
-- everywhere: Go zero value, omitted frontmatter key, '' column.

-- entities: the pointer joins the identity.
ALTER TABLE entities ADD COLUMN pointer TEXT COLLATE "C" NOT NULL DEFAULT '';
ALTER TABLE entities DROP CONSTRAINT entities_pkey;
ALTER TABLE entities ADD PRIMARY KEY (id, pointer);

-- The case-insensitive identity rule (0007, BUG-3RCWNS) constrains which
-- BARE ids may coexist; states of one id legitimately share lower(id),
-- so the uniqueness widens to (lower(id), pointer). Note what the index
-- does and doesn't enforce: cross-entity case collisions on the SAME
-- pointer slot are rejected here, but ('ABC','') alongside ('abc','draft')
-- is rejected by the write path's family probe (a state requires ITS
-- OWN default row, and 'abc' has none) — the same division of labor as
-- fs/mem, where the store-level check is the family scan.
DROP INDEX entities_id_lower_key;
CREATE UNIQUE INDEX entities_id_lower_key ON entities (lower(id), pointer);

-- relations: the state-specific TAIL joins the identity (§2.3: heads
-- stay entity-level, so there is deliberately no to_pointer).
ALTER TABLE relations ADD COLUMN from_pointer TEXT COLLATE "C" NOT NULL DEFAULT '';
ALTER TABLE relations DROP CONSTRAINT relations_pkey;
ALTER TABLE relations ADD PRIMARY KEY (from_id, from_pointer, rel_type, to_id);

-- Derived unique indexes (rela_derived_uniq__*, TKT-3Q0GP1) become
-- DEFAULT-STATE-ONLY: `unique: true` is a natural-key rule over the
-- default world, and a copied state sharing its family's value must not
-- violate it. The index NAME encodes only (type, property) — not the
-- DDL — so the boot reconcile's IF NOT EXISTS would silently keep an
-- old pointer-unaware definition. Each index is therefore REBUILT HERE,
-- inside the migration transaction, by appending the narrowed predicate
-- to its own recorded definition:
--
--   * The constraint is never absent: drop+recreate commit atomically,
--     so no write can slip a duplicate through a gap. (Deferring the
--     recreate to the boot reconcile would open exactly that gap — the
--     reconcile is best-effort by design and swallows failures, and a
--     duplicate written in the window would make the recreate fail
--     forever after.)
--   * The recreate cannot fail on existing data: the new predicate is
--     strictly narrower (old-predicate AND pointer = ''), and every
--     existing row satisfies pointer = ''.
--   * The reconcile then finds the name present and keeps this
--     definition, which is semantically identical to what it would
--     build (see derivedschema.go's DDL, which carries the same
--     pointer = '' clause since this migration's release).
--
-- A derived index without a WHERE clause would make the append invalid
-- SQL and fail the migration loudly — correct, since every index under
-- this prefix is created with one (derivedschema.go) and anything else
-- is not ours to rewrite silently.
DO $$
DECLARE idx record;
BEGIN
    FOR idx IN
        SELECT indexname, indexdef FROM pg_indexes
        WHERE schemaname = current_schema()
          AND indexname LIKE 'rela\_derived\_uniq\_\_%'
    LOOP
        EXECUTE format('DROP INDEX %I', idx.indexname);
        EXECUTE idx.indexdef || ' AND (pointer = '''')';
    END LOOP;
END $$;
