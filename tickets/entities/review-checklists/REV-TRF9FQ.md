---
id: REV-TRF9FQ
type: review-checklist
title: 'Review: Lua write bindings cannot name a face, so a faced type is uncreatable from a script'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` — green; only pre-existing macOS linker
warnings from `cmd/rela-desktop`)
- [x] Lint clean (`just lint` — 0 issues)
- [x] Comment lint gate clean (`just comment-lint` — no unresolvable doc links
across 15,342 comments)
- [x] Coverage maintained (`just coverage-check` — package and total
thresholds PASS, 79.7% total)

Also run, since this change touches a boundary and a generated tree: `just
arch-lint` (OK), `just plimsoll` (clean), `just docs-check` (✓ up to date — the
CI failure RR-NP9T1J predicted, confirmed avoided).

**Comment findings:** `just comment-report` reports no advisory findings in any
of the new or modified files. Two `doclink` findings my first draft introduced
were **fixed, not suppressed**: Go cannot link an unexported member, so
`[Deps.requireCreateFaceFor]` and `[Manager.authorizeAndAudit]` lost their
brackets.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviewers ran in parallel: general code review and a security review scoped
to the face-as-authorization-coordinate question.

**Review Responses:** RR-HQUW7V (critical), RR-7SJ7KN, RR-EXIC51, RR-2775B7
(significant), RR-BC2C46, RR-W3R64J (minor, addressed), RR-0CRSC9 (minor,
deferred → TKT-BJ7H82).

**The critical one is worth stating plainly.** My `requireRelationFaceFor`
*required* a face on a content-scoped edge from a faced source. RR-9LM7T7 had
asked only that an invalid face be *rejected*; requiring one was scope I added
without surveying the callers. It broke `rela link`, the MCP `create_relation`
tool (whose schema has no face parameter at all), CalDAV membership writes,
`rela renumber`, and the data-entry incoming-edge path — whose zero tail is a
documented deliberate decision. **The entire test suite stayed green**, because
nothing else exercises a content-scoped relation from a faced source. I
reproduced it against the real manager before fixing.

Fixed by dropping the requirement and keeping both rejection rules, with the
asymmetry documented at the declaration and pinned by
`TestCreateRelation_ZeroTailStillWorksForFacelessCallers`, which reproduces the
exact option structs `cli/link.go` and `mcp/tools_relation.go` pass. I verified
that guard fails against the old behaviour.

**Self-review:** the only changes outside the ticket's stated scope are the
`mockManager` fix the plan itself required and one doc comment in
`internal/datamigration`. No unrelated edits, no TODOs, no debug code.

The security review confirmed the three claims the design hinged on, by probing
a live manager + `memstore` + real `acl.Declarative` rather than by reading: the
check runs before `authorizeAndAudit` on both paths and does execute under
`bypassACL`; a script naming a face it lacks a grant for is denied
(`GrantsVerbOnState` is exact-match); and both `return nil` fail-open-looking
branches are backstopped by `ValidateRelation` and the peer-existence check,
with zero relations landing in every probe.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status** — evidence in IMPL-KFWMLW; all verified by automated test
*and* by running real scripts against a faced project, with the result confirmed
at the storage layer rather than from the binding's own report.

| AC | Status | Evidence |
| --- | --- | --- |
| 1 | PASS | `{face="draft"}` → row at `entities/policys/POL-1@draft.md`; `TestCreateEntity_FaceReachesTheManager` asserts the value at the manager boundary |
| 2 | PASS | No opts table → `a create must name one: policy declares draft, published`; nothing written |
| 3 | PASS | Faceless + face → `source declares no faces, so "draft" names nothing`; no opts → unchanged |
| 4 | PASS | `{face="nope"}` → `does not declare this content state` |
| 5 | PASS | `{face=42}`, `{fce="draft"}` and a bare-string opts argument all raise, with **zero** manager calls (RR-7SJ7KN strengthened this assertion) |
| 6 | PASS | Content-scoped edge at `relations/POL-1@draft--cites--SRC-1.md` |
| 7 | PASS | Identity scope refuses a face from **both** bindings — `TestElevatedCreateRelation_TakesAFace` closed the elevated gap (RR-2775B7) |
| 8 | PASS | 2/3/4-arg entity creates and 3-arg relation creates unchanged; the arg-4 break is declared, with zero in-tree callers |

**Mutation-tested.** A passing suite proves nothing unless it can fail, so four
deliberate reversions, each confirmed red and restored: the non-table guard
(failed on exactly the case the design review predicted), the manager check, the
`Face:` field, and the zero-tail acceptance.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-14QHDU

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Verified end to end by a developer-shaped path rather than by unit tests alone:
built `cmd/rela`, wrote ordinary scripts against a project declaring `faces:`,
and confirmed both the happy paths and all six refusals.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR
post-dates this checklist — `/pr` gates on the ticket already being `done`,
so this item can only be satisfied by a PR that does not exist yet. See
TKT-UFV01M and the note below.)
