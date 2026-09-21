-- pgstore schema, version 16: the data-migration record (TKT-XCJ0Y2).
--
-- Backs internal/datamigration/pgmigstate. Replaces the state_kv key the
-- migration marker used to occupy, for the reason that split comments and
-- versioning out of state_kv: "is this about the machine or about the
-- content?"
--
-- state_kv is node-local runtime state — the render cache, user settings, the
-- operator's logo. This record says which migrations have run against the
-- CONTENT and what schema shape that content conforms to, so it belongs with
-- the content it describes.
--
-- Rows live in the tenant's schema like every other table here, so a
-- schema-per-tenant deployment keeps the guarantee docs/postgres-backend.md
-- already documents: each tenant tracks its own shape and tenants at
-- different points migrate independently.
--
-- # One row
--
-- The id CHECK pins a single row. A schema IS one store, so a key column
-- would have exactly one possible value.

CREATE TABLE migration_state (
    id    integer PRIMARY KEY CHECK (id = 1),
    state jsonb   NOT NULL
);
