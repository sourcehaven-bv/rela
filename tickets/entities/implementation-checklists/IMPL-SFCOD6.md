---
id: IMPL-SFCOD6
type: implementation-checklist
title: 'Implementation: Typed state references and the store contract (Step 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Multi-PR ticket** (3-PR stack, see PLAN-WCAXRG + `.ignored/dofyr1-plan.md`).
Status per PR is tracked here; the checklist completes with the stack.

- **PR-A** https://github.com/sourcehaven-bv/rela/pull/1386 — contract core +
fs/mem + storetest. All 24 CI checks green. Code review: 1 critical + 5
significant, ALL fixed same-session (RR-LG8QUB, RR-5GI8XO, RR-2Y3L58, RR-QOJTMJ,
RR-NTKQVY, RR-EPVQJB); minors RR-TV1DM6 addressed, RR-GSOQY1 deferred (O(n)
family scans, follow-up).
- **PR-B** https://github.com/sourcehaven-bv/rela/pull/1388 — pgstore
native states (migration 0011 with in-tx derived-index rebuild, compound PK, FOR
SHARE family probe, codec wire boundaries, default-world scoping for
search/manifest/versioning/graph); transitional Capabilities.States flag
DELETED, States suite now unconditional. All 24 CI checks green; local DB-gated
race suite green. Code review: 3 criticals (pg observer divergence RR-ZI5ZLB,
probe race RR-HRC7JC, rename panic RR-7081BE) + 2 significant (migration
enforcement gap RR-0O6030, cursor garbage-compare RR-HVUBS7) + minors RR-L2PXEH
— ALL fixed with pins.
- **PR-C** (closing PR) — relation `scope:` declaration (validated,
declarative-only, IsIdentity/IsContent accessors), content-state detection
(`CheckStates` findings: undeclared-pointer / headless-family /
state-type-mismatch; `rela analyze states`; summary
  + JSON envelope; appbuild boot warning with advisory-count semantics),
docs (metamodel guide + generated docs), mcp golden regenerated (additive Scope
field). Code review: 4 significant + 6 minor, ALL fixed (RR-BYK5TO, RR-RACANY).
Lands the ticket paperwork.

## Development

- [x] Unit tests written for new code (codec tables; storetest RunStateTests: addressing, invariants, query scope, relation tails, cascades, events, opacity, aggregates)
- [x] Integration tests written (fsstore state persistence across reopen; observer-skip incl. headless rename; manual CLI e2e on a project with state files)
- [x] Happy path implemented (PR-A: fs/mem full contract; PR-B pending for pg)
- [x] Edge cases from planning handled (configured/whitespace grammar cases, headless tolerance, family rename/delete, indexed relation query, unusual-canonical opacity)
- [x] Error handling in place (row-family invariants reject with named reasons; pg transitional fails closed on writes AND AllStates reads; codec errors name their cause)

## Test Quality

- [x] Using fixture builders or factories for test data (storetest helpers; shared appbuild/fsstore/memstore fixtures; hand-authored files only where the shape is write-path-rejected by design)
- [x] No hardcoded values in assertions when object is in scope (full-struct violation/finding comparisons; codec round-trips)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- PR-A: built rela; a project with `PAGE-1@draft.md` + a state-tailed
relation file reads as exactly today's 2-entity default world (list, show,
analyze all clean).
- PR-B: migration 0011 applied via psql to a real 0001→0010 database
with seeded data + a derived unique index — rebuilt predicate correct, state row
sharing its family's unique value inserts, duplicate default rejected; full
DB-gated race suite (`just test-postgres`) green against a locally provisioned
postgres.
- PR-C: boot warning fires with project root + advisory count;
`rela analyze states` reports `[undeclared-pointer] draft: 1 row(s) … (e.g.
PAGE-1@draft)`; `scope: content` parses; `scope: per-state` is a load error
naming the allowed values.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (codec allowlist grammar; @ rejected everywhere user input enters; fail-secure cascades kept; pg fail-closed window; FOR SHARE race fix; search/manifest default-world scoping keeps drafts invisible)
- [x] No silent failures (CheckStates/CheckCardinality fail loudly; probe errors traced at Debug; cursor degrades documented and honest)
- [x] No debug code left behind

## Quality Gates (final, 2026-08-20, per PR)

Each PR: `go test ./...`, `just lint` (0 issues), `just plimsoll`, `just
arch-lint`, `just coverage-check` (78.1–78.2%), gofmt; PR-B also `just
test-postgres` (race, DB-gated) + `go build -tags postgres`. CI: PR-A #1386 and
PR-B #1388 all 24 checks green; PR-C pending at paperwork time.
