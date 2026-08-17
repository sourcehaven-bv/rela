---
id: RR-2HMGZJ
type: review-response
title: Drop-extra introspection must filter by current_schema() or it cross-schema-deletes
finding: pgstore SQL is unqualified (search_path isolation). pg_indexes shows all visible schemas, so a shared-DB multi-schema deploy would have schema A's reconciler DROP schema B's rela_derived_% indexes (owned-but-not-declared-in-A). Introspection must constrain to schemaname = current_schema().
severity: significant
status: open
---

All pgstore SQL is unqualified — isolation is purely via search_path.
Introspecting `rela_derived_%` via pg_indexes/pg_class is NOT automatically
schema-scoped: pg_indexes shows indexes across all visible schemas. If two
schemas share a DB (a supported topology — the change-feed channel is DB-global
filtered by schema, feed.go:28-53), schema A's reconciler enumerates schema B's
rela_derived_% indexes and, being "owned but not declared in A's metamodel,"
DROPs them. feed.go documents this exact hazard class.

REQUIRED: constrain introspection to `schemaname = current_schema()` (or
`relnamespace = current_schema()::regnamespace`). The "owned namespace"
reasoning is necessary but not sufficient without the schema filter.
