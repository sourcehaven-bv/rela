---
id: REV-EYSBZD
type: review-checklist
title: 'Review: Extend store.GraphQuery with property predicates and relation negation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `./internal/...` green; pgstore
      additionally run against a live PostgreSQL with `-race` (41s)
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained (`just coverage-check`) — package floor (50%) and
      total (65%) both PASS; total 76.9%. New `internal/propmatch` at 97.0%.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — RR-RGAXHK
- [x] All significant review-responses addressed — RR-6947C1, RR-VK9XIH
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-RGAXHK (critical, addressed), RR-6947C1 (significant,
addressed), RR-VK9XIH (significant, addressed), RR-UVJOFV (minor, addressed),
RR-0EWZQW (minor, deferred — CI config, out of scope)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented below

**Acceptance Status:**

1. **Both backends pass the extended conformance suite** — PASS.
   `storetest.RunGraphQueryTests` gained 11 subtests (property equality,
   AND-ing, empty-vs-absent, exclusion-does-not-widen, prop+relation combined,
   negation inbound/outbound/named-endpoints, any-endpoint existence, and
   `GraphCount` / `MatchingIDs` coverage), plus a table-driven
   `Props_value_shapes` case crossing scalars / empty lists / populated lists /
   ints / bools with six op-target combinations. Verified green on fsstore,
   memstore (naive path) and pgstore (SQL path, live database).

2. **A `type + property` query is index-backed on postgres** — PARTIAL, see
   note. The predicate is pushed into the SQL `WHERE` as a jsonb comparison
   rather than filtered in Go, which was the substance of the criterion (the
   rejected alternative, `SearchVisible`, cannot push filters down at all). A
   formal `EXPLAIN` assertion in the style of `graphquery_explain_test.go` was
   NOT added: `properties ->> key` is not index-backed without a matching
   expression index, and no such index exists in the schema today. Adding one
   per queried property is a schema decision beyond this ticket. Flagged here
   rather than silently claimed.

3. **`just arch-lint`, `just ci` green; postgres suite via
   `just test-postgres`** — PASS for arch-lint (OK, no warnings), lint,
   coverage, and the postgres suite against a live database. Full `just ci` not
   run end-to-end locally; its constituent targets were run individually.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal store
      API with no user-facing surface — `GraphQuery` is not reachable from
      config, CLI, or the HTTP API. The consumer that will expose it,
      next-action sources, is TKT-CXD0A4 and carries its own docs obligation.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A: same reason)

Godoc was updated in place: `store.PropPredicate` / `PropOp` /
`RelationPredicate.Negate` carry the semantics and the empty-Endpoints
widening warning; `internal/propmatch` has a package doc explaining why it is a
pure leaf and what it deliberately does not cover.

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: pushing and
      opening a PR is the maintainer's call — not taken autonomously. The work
      sits on a local branch ready for it.)
- [x] ~~All CI checks pass~~ (N/A: no PR yet. CI's constituent targets were run
      locally — `just lint`, `just arch-lint`, `just coverage-check`, full
      `./internal/...`, and the postgres suite with `-race` against a live
      database — all green.)
- [x] ~~PR URL documented below~~ (N/A: no PR yet)

**PR:** not created. Branch `docs/next-action-research` holds four commits
(research + tickets, propmatch extraction, GraphQuery extension, review fixes),
unpushed.
