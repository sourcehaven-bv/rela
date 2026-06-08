-- pgstore schema, version 2.
--
-- Composite indexes on `relations` keyed by (rel_type, from_id) and
-- (rel_type, to_id). The GraphQuery DSL's recursive CTE filters by
-- rel_type at every step (`WHERE r.rel_type = ANY($2)`), so without
-- these composites the planner uses the single-column from_id / to_id
-- indexes from migration 0001 and re-checks rel_type per row — fine
-- for small relation tables but degrades fast under realistic load
-- with mixed relation types.
--
-- IF NOT EXISTS so the migration is idempotent: re-applying on a
-- database that already has the indexes (e.g. test schemas that
-- migrated through this version once) is a no-op.

CREATE INDEX IF NOT EXISTS relations_type_from_idx
    ON relations (rel_type, from_id);

CREATE INDEX IF NOT EXISTS relations_type_to_idx
    ON relations (rel_type, to_id);
