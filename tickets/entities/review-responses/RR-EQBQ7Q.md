---
id: RR-EQBQ7Q
type: review-response
title: Migration 0014 rewrites every entity row inside the migration transaction
finding: 'An unqualified UPDATE of search_text under the migration advisory lock, auto-applied on first open: a full table rewrite plus trigram index rebuild that blocks startup with no progress signal.'
severity: significant
resolution: 'The UPDATE now touches only rows whose search_text IS DISTINCT FROM the new composition (a re-run is cheap), the migration header explains the one-time cost, and docs/postgres-backend.md tells operators to run `rela db migrate` as a deploy step if the pause must not land on the first request. Batching a first-time rewrite is not done: the migration runs in one transaction by design.'
status: addressed
---
