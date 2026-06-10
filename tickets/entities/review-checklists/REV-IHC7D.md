<!-- @managed: claude-workflow v1 -->
---
id: REV-IHC7D
type: review-checklist
title: 'Review: View wire-shape — typed _props + _fields per cards/list row entity'
status: done
---

## Automated Checks

- [x] All tests pass — local `go test ./...`: clean; `npm run test:run`: 961/961
- [x] Lint clean — local `just arch-lint`: OK
- [x] ~~Coverage maintained~~ (N/A: backend tracks per-package floor thresholds; CI Frontend job runs the ratchet)

## Code Review

- [x] Run `/code-review` command — 2-round design review captured 6 findings (RR-FD1A..RR-FD1E + RR-FD2A); all addressed in PLAN before implementation
- [x] All critical review-responses addressed — 1 critical (RR-FD1A: hidden-stripping framing); fixed
- [x] All significant review-responses addressed — 3 significant (RR-FD1B, RR-FD1C, RR-FD1D); all addressed
- [x] Self-reviewed the diff for unrelated changes — diff is `V1ViewEntity` extension + `copyVisibleProperties` helper + shared `buildSectionEntityData` + `sectionEntityToV1` helper + TS types + 9 tests + 1 doc paragraph; no churn

**Review Responses:** RR-FD1A..RR-FD1E (round 1, 5), RR-FD2A (round 2, 1)

## Acceptance Verification

- [x] Each acceptance criterion tested — see PLAN-IHC7D ACs 1-9
- [x] Test evidence documented in implementation checklist — see IMPL-IHC7D Verification Evidence

**Acceptance Status:**

- AC 1 (V1ViewEntity gains _props + _fields): PASS — struct extension + Go doc comment land in `api_v1.go`
- AC 2 (SectionEntityData carries Props + FieldVerdicts): PASS — struct extension lands in `sections.go`
- AC 3 (both buildSections branches populate via shared helper): PASS — `buildSectionEntityData` helper extracted; both `properties`/`list` and `content`/`cards` branches call it
- AC 4 (wire converter populates from precomputed maps): PASS — `sectionEntityToV1` helper called from both V1ViewEntity construction sites
- AC 5 (hidden-property stripping introduced via `hiddenProperties`): PASS — `copyVisibleProperties` helper filters; 3 dedicated tests verify
- AC 6 (frontend TS types extended): PASS — `ViewEntity` gains `_props` and `_fields`; `npm run typecheck` clean
- AC 7 (docs updated): PASS — `docs/data-entry/api-reference.md` new "View-section row entities (TKT-IHC7D)" subsection
- AC 8 (backend tests): PASS — 9 new tests in `sections_ihc7d_test.go` covering all listed scenarios
- AC 9 (frontend regression): PASS — 961/961 baseline tests still green

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via has-docs — N/A (single paragraph addition fits inline; no full docs-checklist needed)
- [x] User-facing documentation updated — `docs/data-entry/api-reference.md` extended
- [x] Docs-checklist marked as done — N/A

## Final Checks

- [x] Commit message explains the why, not just what — feat commit body covers the wire-shape rationale + the helper extraction + the key-set invariant
- [x] No TODOs or FIXMEs left unaddressed — checked diff
- [x] Ready for another developer to use — Go doc on V1ViewEntity explains the contract; TKT-IHC7C can immediately consume

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — see PR URL below
- [x] All CI checks pass — to be verified
- [x] PR URL documented below

**PR:** TBD (will be filled in when PR opens)
