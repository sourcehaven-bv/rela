---
id: TKT-GIAGLQ3
type: ticket
title: Push ingest find and rename dry-run lookups into the store
kind: enhancement
priority: medium
effort: m
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

TKT-FED6FMM left these `request-path` entries in
`internal/archguard/unpagedread_allowlist_test.go`:

- `internal/ingest/ingest.go#findTarget`: scans the find type for the record match values, per webhook or subscription record. `visibility.HolderLookup.Headers` already answers a gated, paged multi-equality query; the ingest `Deps` need a finder built from it at the two wiring sites (`dataentry/webhook_routes.go`, `scheduler/subscriptions.go`). Decide how non-string match values compare (`propString` uses `%v`; `headerMatches` compares strings only).
- `internal/entitymanager/rename.go#idTakenByOther`: the rename dry run scans every header for a case-folded id collision. No store query folds ids. Add a store capability (for example `IDTakenFolded`) on all four backends with storetest conformance; pgstore has the `entities_id_lower_key` index.
