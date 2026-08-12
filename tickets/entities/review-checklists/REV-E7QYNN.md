---
id: REV-E7QYNN
type: review-checklist
title: 'Review: CalDAV: go-webdav adapter, VTODO collections under /api/, getctag + two-way PUT/DELETE'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` — **0 issues** (also fixed a pre-existing `funlen` on
`appbuild.assemble`, extracted as `resolveACL`). `just arch-lint` — OK.

Tests: all CalDAV packages pass. 14 failures remain in `internal/docscapture`
and `internal/lua`, unrelated to this work and **present on clean `develop`**
(verified by stashing); they parse a `data-entry.yaml` fixture with an unrelated
schema drift.

Coverage: package floor **PASS**, total floor **PASS**, total 77.3%.
Per-package: caldavalias 95.7%, calfeed 95.4%, dataentryconfig 89.3%,
dataentry 80.0%.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

The review found **five critical defects, all reproduced** — and every one was a
data-loss bug that the green unit-test suite did not catch, because the tests
covered individual operations while a sync protocol lives in *sequences*
(PUT → LIST → GET → PUT, and edits arriving from outside CalDAV).

Each fix was independently re-verified against the live Reminders demo, and each
regression test was checked to FAIL against a simulated reintroduction of the
bug before being kept.

**Review Responses:**

| ID | Severity | Status | Finding |
|----|----------|--------|---------|
| RR-6P8QL8 | critical | addressed | If-Match compared against a cached ETag only CalDAV writes refreshed — 412'd valid writes AND accepted stale ones |
| RR-B4RZRA | critical | addressed | Soft delete dropped the alias; a replayed PUT created a second entity |
| RR-R4SCVX | critical | addressed | COMPLETED without STATUS silently dropped — the checkbox reverted |
| RR-27WGOX | critical | addressed | A corrupt gitignored cache file bricked every `rela` command |
| RR-I4FN1T | significant | addressed | `LookupByEntity` map-order nondeterminism flipped the served href between polls |
| RR-8UOVDH | significant | deferred | `where:` not applied on writes (type-confusion half fixed; filter half needs a design decision) |
| RR-3UAG12 | significant | deferred | Retained aliases grow unbounded; whole-table rewrite per write |

Both deferrals are recorded with reasoning on the entities and as follow-ups on
the tickets. Neither is a regression: RR-8UOVDH is not an ACL bypass (the write
is still authorized by entitymanager), and RR-3UAG12's whole-table rewrite
predates this branch — only its growth rate changed.

**Unrelated changes in the diff:** two, both deliberate and called out in their
commits — the `appbuild.resolveACL` extraction (needed to get `just lint` green)
and the `AliasRewriter.EntityDeleted` contract correction (its doc stated the
inverse of the behaviour after the tombstone change).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1. Reminders sees every configured collection from one account URL | PASS | Live: accountsd discovery captured on the wire (`/.well-known/caldav` → `PROPFIND /` → home set); the `tasks` list appears in Reminders |
| 2. Checking off writes back, losing no other property | PASS | `PatchEntity` throughout; `internal_ref` survives a client write (demo TSK-milk). Reviewer independently confirmed redacted props survive |
| 3. Creating a to-do creates an entity of the collection type | PASS | Live PUT → 201 → new entity; `TestCalDAV_StaleWriteInferredFromServerStateAlone` |
| 4. Deleting applies the configured transition | PASS | Live DELETE → 204, `status: cancelled`, unlisted; `TestCalDAV_SoftDeleteKeepsTheAlias` |
| 5. A conflicting PUT (stale If-Match) returns 412 | PASS | **Was broken — RR-6P8QL8.** Now: fresh → 201, stale → 412, verified live and by `TestCalDAV_IfMatchComparesAgainstCurrentContent` |
| 6. getctag changes when a member changes, stable otherwise | PASS | Live: stable across polls, changes on edit, changes on delete, returns to the original tag on revert (so genuinely content-derived) |
| 7. Reads ACL-scoped; a hidden entity is absent, not 403 | PASS | Routed through `feedEntitySource`; reviewer verified no ctag existence-oracle between principals |
| 8. `*ical.Calendar` confined to the adapter; arch-lint passes | PASS | `just arch-lint` OK |
| 9. Verified against Reminders / Thunderbird / eM Client / Cfait | **PARTIAL** | Reminders (macOS) verified end-to-end on real hardware. The other three are **not tested** — see below |

**AC9 is not met and should not be marked as such.** Only Apple Reminders has
been exercised. This matters less than it did earlier in the branch: the
deletion logic no longer depends on client x-property preservation (that design
was reverted — see the TKT-MF1CWZ notes), so no *correctness* property now rests
on untested client behaviour. What remains untested is ordinary interop.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: the
user-facing documentation is the deliverable of its own ticket, **TKT-N8RESF**,
which carries its own review checklist — a second checklist over the same
artifact would duplicate it)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: same as above)

`docs/caldav.md` — deployment guide (topology, collection config, Pratique
setup, credential issuance, the macOS client steps, deletion semantics,
constraints), linked from the README docs table. Claims verified against code
rather than assumed: every rela flag checked present in `cmd/rela-server/main.go`,
Pratique's TLS confirmed merged on its `develop`, and the JWKS path corrected to
`/.well-known/pratique/jwks.json` after an earlier draft placed it under the
mount prefix.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1308
