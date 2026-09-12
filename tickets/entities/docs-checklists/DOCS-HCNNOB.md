---
id: DOCS-HCNNOB
type: docs-checklist
title: 'Docs: store: paged variant of GraphQueryer (GraphQueryPage) — pgstore SQL pushdown + naive early-stop + conformance'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc written for all new exported symbols

Godoc is the primary documentation deliverable for this ticket, since the change
is an internal store contract with no user-facing surface. New/updated:

- `store.GraphPageQuery` — states *why* paging lives on its own type rather than
as extra fields on `GraphQuery`: `GraphCount` must count the whole matched set
and `MatchingIDs` must answer for every candidate id, so neither *can* honor a
limit. Fields they silently ignored would be a quiet ACL footgun (RR-245QB2).
- `store.GraphQueryer.GraphQueryPage` — the full cursor contract (opaque cursor,
`NextCursor` non-empty iff more, `Limit == 0` = one full page, malformed cursor
is an error and never a silent restart), plus an explicit note that the page
carries no total and `GraphCount` is the counting API.
- `graphquerynaive.Reader.ListEntities` — the ascending-id **precondition**
`RunPage` depends on, recorded on the interface rather than in prose on the
function, because `Reader` is a consumer-side interface that deliberately does
not name `store.EntityReader` (RR-R713PD).
- `graphquerynaive.RunPage` — early-stop behavior, and why errors abort the page
rather than skipping the entity (a short page with an empty cursor is
indistinguishable from end-of-results, so skipping silently truncates an
ACL-filtered walk).
- `storeutil.FinishPage` — the limit+1 lookahead convention, including why the
cursor keys on `items[limit-1]` and never on the lookahead row.
- `pgstore.buildGraphQuerySQL` — keyset parameter, the `COLLATE "C"` reasoning
that makes the keyset agree with Go's byte comparison, and an extended
SQL-injection-safety block naming the cursor as another parameterised value.

- [x] Complex logic has explanatory comments
- [x] ~~Public API docs~~ (N/A: `internal/` package, no public API)

## Project Documentation

- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command added or changed)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI or HTTP surface change)
- [x] ~~`docs/transforms.md` / backend guides~~ (N/A: no transform or backend-behavior change)
- [x] ~~`README.md`~~ (N/A: no project-level change)
- [x] `CLAUDE.md` reviewed — no update needed

Checked the root `CLAUDE.md` store section deliberately rather than by default.
It documents store contracts, the storetest conformance requirement, and the
build-tag rules; this change adds a method that follows those existing rules
rather than establishing a new one. The one architectural fact worth recording —
that `graphquerynaive` may depend on `storeutil` — is captured where it is
enforced, as a comment on the `.go-arch-lint.yml` rule, which is the file that
would reject a future violation.

- [x] Stale documentation corrected

Reporter note 1: `internal/store/graphquery.go:17` claimed "A future
SQL-pushdown implementation in pgstore is tracked as a follow-up." That shipped
long ago — pgstore has been SQL-native (recursive CTE + `WHERE EXISTS`).
Corrected to describe the actual split: fsstore/memstore delegate to
`graphquerynaive`, pgstore implements the contract SQL-natively including keyset
paging. The reporter noted they nearly planned around the stale claim, which is
exactly the cost of leaving it.

## External Documentation

- [x] ~~Release notes / migration guide~~ (N/A: no behavior change for any
existing caller. `GraphQuery`, `GraphCount`, and `MatchingIDs` are untouched;
`GraphQueryPage` is additive and has no production consumer yet.)
- [x] ~~User-facing docs~~ (N/A: internal interface; nothing in `docs/`
describes `GraphQueryer`, and no Lua, CLI, MCP, or HTTP surface changed. The
consumer-visible change lands with TKT-YWDGZD.)
