---
id: DOCS-CF2ZL2
type: docs-checklist
title: 'Docs: PostgreSQL read-path performance audit (query accounting, perf seed, batching, pushdown, indexes)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

New exported API carries godoc that says why it exists, not just what it does:
`store.QueryStats` (why two counters and nothing per statement),
`store.RelationQuery.EntityIDs` (nil vs empty, composition with Direction and
FromFace), `store.GraphHeaderQueryer` / `GraphQueryHeaders`,
`store.MatchedCounter` / `CountMatched` (why the unscoped total is never
exposed), `store.GraphQuery.OrderBy/Limit/Offset` and `OrderSpec` (absent value
sorts as largest, and why that asymmetry lets one index serve both directions),
`store.DerivedListIndex` and `DerivedObjectSpec.OrderBy`,
`worldreader.RelationReader.NeighborsForPage`,
`userstate.Store.SnoozedUntilMany` / `LastShownMany`, `queryplan.StringShaped`,
`acl.PrincipalStore`, `storetest.Counting`, the whole `perfseed` package
(package doc explains why a generator, determinism, and the raw-store
obligations).

Decisions documented where someone would otherwise undo them:
`pgstore/tracer.go` (always attached; no allocation when nothing applies),
`dataentry/requeststats.go` (Debug gate is a security property),
`dataentry/rowcontent.go` (file comment: where content-free entities may and may
not travel), `dataentry/listpushdown.go` (eligibility is what makes the two
paths agree), `pgstore/world.go effectiveWorld` (byte-identical collapse),
`pgstore/search.go searchRankPrefix` (ranking on the identity/title prefix),
migration `0014_read_indexes.sql` header (each index's reason, the tsv drop, the
search_text rebuild and its lower() caveat), `pgstore.go` plimsoll note
(optional capabilities must live on the store type to be type-assertable),
`acl/declarative.go requestFor` (why reuse is safe only with both guards).

## Project Documentation

- [x] CLAUDE.md updated with new patterns — the "collection reads are
content-free, batched per page, paged in the store" rule; the fourth raw-store
exception (`rela dev seed`)
- [x] docs/ updated for changed behaviour — `docs/postgres-backend.md`
("Observing query cost", derived list indexes, corrected search paragraph,
ordering semantics), `docs/data-entry.md` (sort semantics for absent values),
`docs/data-entry/api-reference.md` ("Collection rows are content-free",
`include_content`), `docs/acl-security.md` ("Query accounting is a Debug-only
diagnostic")
- [x] ~~Architecture docs updated~~ (N/A: no package boundary change beyond the
new `perfseed` leaf, which is declared in `.go-arch-lint.yml` with its reason)

## External Documentation

- [x] ~~README updated~~ (N/A: no headline feature; the seeder is a developer tool)
- [x] CLI reference updated — `docs/cli-reference.md` gains `rela dev seed`
- [x] API docs updated — OpenAPI list parameters gain `include_content`; the
API reference documents content-free rows
