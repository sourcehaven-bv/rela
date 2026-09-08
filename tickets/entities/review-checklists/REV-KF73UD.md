---
id: REV-KF73UD
type: review-checklist
title: 'Review: Aggregate-over-hidden-rows documents: elevated document renders whose output is a derived statistic'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->

## Acceptance criteria verification

Each AC from PLAN-1DETM0 with the named test that pins it. All passing.

| AC | Property | Test |
|---|---|---|
| 1 | Read-only handle exposes no write methods | `TestElevatedRead_ReaderOnlyHandle` (lua) |
| 2 | Cascade path unchanged (both handles) | `TestRun_ElevationRequiresBothKeys` (script), full `internal/lua` suite green |
| 3 | Nil reader is a DENY, not a fallback | `TestElevatedRead_DeniesWhenNoReaderWired` |
| 4 | Elevated doc renders for the permission holder | `TestElevatedDocument_PermittedPrincipalRenders`, `TestElevatedRender_ReadsHiddenEntityAndAudits` |
| 5 | Unelevated doc has no `bypass_acl` at all | `TestUnelevatedRender_CannotReachHiddenEntity`, `TestElevatedDeps_GrantsBypassBinding/unelevated_render_has_no_bypass_acl` |
| 6 | Elevated + no permission is a config error | `TestValidateConfig_Documents/elevated_without_permission_is_an_error` (+5 sibling cases) |
| 7 | Closed switch over every ACL impl, incl. face forms | `TestElevatedDocument_ClosedSwitch` (7 cases) |
| 8 | NopACL does not serve an elevated doc | `TestElevatedDocument_NopACLDeniesDespitePermittingGate`, `TestElevatedDocument_NopACLRefusesEndToEnd` |
| 9 | Denied principal never reaches the renderer | `TestElevatedDocument_DeniedPrincipalNeverReachesRenderer` (asserts `callCount() == 0`) |
| 10 | One `acl-bypass-read` row per elevated render | `TestElevatedRender_ReadsHiddenEntityAndAudits` |
| 11 | Audit survives a raising closure | `TestElevatedRead_AuditsEvenWhenClosureRaises` (lua; path is surface-independent) |

Two of these were only made real by the code review — see RR-DOCAUDIT and
RR-DOCWIRE. AC4/AC10 were previously "verified" by tests rendering through a
fake with `s.elevation == nil`, which passed whether or not elevation worked.
The replacements use a `DenyReader` as the ordinary reader so the hidden entity
is reachable ONLY through elevation, and were mutation-tested (disabling
`elevatedDeps` makes them fail with the expected diagnostic).

## Review summary

`cranky-code-reviewer` run on the full `develop...HEAD` diff. 7 findings, all
addressed: 1 critical, 3 significant, 3 minor.

The critical one (RR-DOCWRT) was a false claim I made in five places — that a
document render is structurally unable to mutate. It is not: renders run on a
WriterRuntime with no `isDocument` guard on the write bindings, so a GET can
create an entity. Pre-existing (TKT-PX5YL7, filed earlier this session) but this
branch asserted the stronger property as fact, which is exactly what the root
CLAUDE.md gate rule forbids. All five claims corrected to "cannot write past the
ACL", which is true and still worth having.

The reviewer independently confirmed the gating is sound: closed switch holds,
no ungated render path exists (five render call sites checked repo-wide), the
unmarshaller resists 21 input shapes, the migration is idempotent, and the
permission requirement is correctly scoped to elevated documents only.

## Automated checks (re-run against the final diff)

- `just test` — all green
- `just lint` — 0 issues
- `just arch-lint` — OK (caught and fixed a metamodel import into autocascade)
- `just plimsoll` — OK
- `just coverage-check` — 77.7%, both thresholds satisfied
- `markdownlint` — 0 issues across 47 files
- `just docs` — regeneration is a no-op against the committed docs

## PR

https://github.com/sourcehaven-bv/rela/pull/1366

Opened against `develop`. The first push produced no CI run at all: the branch
was CONFLICTING (develop had moved 10+ commits, including TKT-X06LA2 #1341
touching the same files), and GitHub does not test an unmergeable branch — the
green CodeQL checks made it look healthier than it was. Merged develop in
(2 conflicts, both ticket metadata, both resolved to develop's version since
that work had landed independently), re-ran the full local gate on the merged
tree, and CI then ran for real.

The `Rela Tickets` job failed on the first real run. Reproduced locally with the
job's exact command (`rela validate --check cardinality --check properties
--check validations` — not the bare `validate` I had been running, which passes
regardless):

1. `RES-XZBZXB` was missing its `summary` property — a real gap, now filled.
2. Five foreign entity files (IMPL-8CWFBK, TKT-YH52OM and relations) had been
swept into commit 89593597 by a careless `git add tickets/`. They belong to the
action-gate work that has since merged to develop on its own. Removed.
