---
id: REV-Y4YC3
type: review-checklist
title: 'Review: Data-entry create form: prefix picker for multi-prefix types and manual ID field'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — Go `go test ./...` green; frontend `npm run test:run` 498/498 passing.
- [x] Lint clean (`just lint`) — Go lint passes; e2e ESLint passes (POP-enforcement rule satisfied via new `FormPage` methods).
- [x] Coverage maintained (`just coverage-check`) — `internal/dataentry` package floor still ≥55% after additions.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — produced 24 findings on 2026-04-25.
- [x] ~~All critical review-responses addressed~~ (N/A: zero critical findings)
- [x] All significant review-responses addressed — 6 of 10 fixed in code; 4 deferred with documented reasons (see RR-GQ5S8, RR-R8W1A, RR-SAIFO, RR-O1UMW).
- [x] Self-reviewed the diff for unrelated changes — diff is scoped to TKT-E7NNM acceptance criteria.

**Review Responses:**

Original design-review (pre-implementation, from PLAN-TFQC4):
RR-2R1HG, RR-3GURO, RR-6GPR4, RR-6HR8S, RR-8D8X3, RR-BJW16, RR-KY4RI, RR-M0LIU, RR-M1TIC.

Code-review (post-implementation, 2026-04-25):
RR-T0H90, RR-15SU1, RR-764AR, RR-O0ZGM, RR-GQ5S8, RR-R8W1A, RR-SAIFO, RR-O1UMW, RR-8EVKJ, RR-V6IVL,
RR-ODPMN, RR-7505O, RR-WI06C, RR-V6WV6, RR-1G4EK, RR-87ICD, RR-CH13R,
RR-4J6HA, RR-RQVPW, RR-RYW1R, RR-8G6D7, RR-PROV0, RR-8T3VM, RR-RN5D5.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) — see status block below.
- [x] Test evidence documented in implementation checklist — IMPL-4J1YO covers each AC with the corresponding test.

**Acceptance Status:**

1. Multi-prefix non-manual types show picker — PASS. Pinned by `TestV1Schema_MultiPrefix`, `TestV1CreateEntity_PrefixOverride`, and e2e `Multi-Prefix Create Form > shows prefix picker and creates entity with chosen prefix`.
2. Single-prefix types do NOT show a picker — PASS. Pinned by composable test `is false for single-prefix types` and e2e `does not show prefix picker for single-prefix feature form`.
3. Manual-ID types: editable ID field in CREATE, read-only in EDIT — PASS. Composable tests `is true for manual type in create mode` / `is false for manual type in edit mode`; e2e `Manual-ID Create Form > renders ID input and creates tag with user-supplied ID` and `edit mode does not show prefix picker`.
4. Backend exposes `id_prefixes`; single-prefix types still expose `id_prefix` for back-compat — PASS. `TestV1Schema_SinglePrefix_Compat` asserts both fields populated.
5. Backend create handler validates `prefix` and `id` against entity type — PASS. 17-row `TestValidateCreateIDOpts` covers the matrix; `TestV1CreateEntity_*` covers the HTTP boundary; `TestHandleAPICreateEntity_IDValidation` (added per RR-T0H90) covers the legacy `/api/entities` path.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: documentation impact is one paragraph in `docs/metamodel.md`; no DOCS-xxxx warranted)
- [x] User-facing documentation updated — `docs/metamodel.md` notes the new prefix-picker UX.
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist created)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what — main commit body covers prefix-picker and manual-ID rationale; follow-up commits cover the rebase merge fixes and code-review responses.
- [x] No TODOs or FIXMEs left unaddressed — `grep -rn 'TODO\|FIXME' frontend/src/composables/useEntityIDControls* internal/dataentry/api_v1.go internal/dataentry/handlers_api.go` returns nothing in changed code.
- [x] Ready for another developer to use — the composable is self-contained and the page-object methods on `FormPage` are reusable for future ID/prefix UI tests.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR #555 already open, CI being walked to green.
- [x] All CI checks pass — Lint, Lint Markdown, Test, Frontend, Architecture, E2E, Fuzz, Vulnerability Check, CodeQL all green; Rela Tickets becomes green when this checklist + ticket flip to `done`.
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/555
