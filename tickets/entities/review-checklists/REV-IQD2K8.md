---
id: REV-IQD2K8
type: review-checklist
title: 'Review: Inline entity creation from relation fields in the data-entry edit form'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./...` fully green; frontend 1503 tests / 94
files pass; e2e 236 pass, 0 failures (8 skipped by design)
- [x] Lint clean — `just lint` 0 issues; frontend ESLint 0 errors; also
`just arch-lint` OK and `just plimsoll` (god-object lint) OK
- [x] Coverage maintained — `just coverage-check` PASS, total 77.3%

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

*Design review (pre-implementation), all addressed:* RR-KGCF61, RR-R1I6AD
(critical); RR-Y15LRA, RR-UTURB1, RR-PP3B60, RR-P3CO33, RR-BEMW01 (significant);
RR-3UOH1I (minor).

*Code review (post-implementation), all addressed:*

| ID | Severity | Finding |
| --- | --- | --- |
| RR-CZOHDW | critical | Cmd+Enter was dead — the embedded form skips its own listener and nothing picked the shortcut up, while the button still advertised `⌘↵` |
| RR-S1A1LN | critical | The discard-confirm rendered *underneath* the inline modal (both `.modal-overlay` at z-index 1000, both Teleported), so Escape on a dirty form looked frozen |
| RR-XABO9R | significant | Dirty-tracking sniffed bubbling DOM events, missing relation picks, wizard steps and CodeMirror body edits — a written body was discarded with no prompt |
| RR-PCHN97 | significant | `justCreated` notice leaked across add cycles, pinning "Created X" next to an unrelated target |
| RR-L3XA9N | significant | Sidebar rebuilt the ACL principal scope per entity type — N `member-of` graph walks per app load |
| RR-2OP5SX | significant | The catalog prototype still set the deleted knobs |
| RR-ZANIPH | minor | Weak dry-run mock, modal-stubbing widget tests, `createFormId` never cleared |

The three criticals/significants in the modal shared one root cause — it reached
for the form's state through the DOM instead of asking for it. Fixed
structurally: `DynamicForm` now `defineExpose`s `{ isDirty, isSaving, submit }`,
which resolves all three with less code than the event-sniffing it replaced.

**Self-review note:** the diff contains no unrelated changes. Two prototype
config edits and two doc-file edits are in scope (they documented the removed
knobs). One accidental deletion of pre-existing fixture relations during manual
cleanup was caught via `git status` and restored with `git checkout`.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
| --- | --- | --- |
| AC1 offer when permitted + form exists | **PASS** | `TestInlineCreate_RequiresAForm`; widget tests for both widgets; e2e `offers "+ New" for a target type that has a form`; manual: alice sees 7 types |
| AC2 hidden without permission | **PASS** | `TestInlineCreate_GatedByCreatePermission`; manual: bob (viewer) gets no `inline_create` at all |
| AC3 hidden when no form resolves | **PASS** | `TestInlineCreate_RequiresAForm` (absent type); `TestInlineCreate_EditOnlyFormIsOffered` pins the edit-fallback boundary (RR-KGCF61) |
| AC4 renders the configured form | **PASS** | e2e `opens a real configured form, not a metamodel dump`; manual: authored label "Category Name", its placeholder, help text and markdown body all rendered |
| AC5 create-and-link round trip | **PASS** | e2e for both widgets; manual: `TKT-007 --belongs-to--> CAT-001` persisted |
| AC6 host draft survives | **PASS** | e2e `creating inline links the new entity and preserves the host draft` + the cancel variant; manual: every field intact, no navigation |
| AC7 no cross-form interference | **PASS** | `DynamicForm.embedded.test.ts` (route guard not registered, no document keydown, no `router.push`); `useFormWizard` `syncUrl` |
| AC8 `false` booleans persist | **PASS** | inherited from the real form's commit path (the deleted modal's falsy-strip is gone); `CAT-001` kept its `status: draft` default, which the old modal dropped |
| AC9 validation and warnings | **PASS** | the nested form is `DynamicForm`, so its existing validation/warning suites apply unchanged |
| AC10 nesting capped structurally | **PASS** | `useInlineCreate.test.ts` depth tests; e2e `a nested form does not offer inline create in turn` |
| AC11 create-without-read degrades | **PASS** | `TestInlineCreate_CreateWithoutReadIsOffered`; POST returns the entity via `forWire`, both widgets seed their caches from it |

Every new Go test and the modal suite were **mutation-checked** — reverting the
implementation fails them — so they are load-bearing rather than vacuous, which
is precisely how the placeholder e2e test this replaced managed to pass while
testing nothing.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-XJVZBS

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — monitored on PR #1295; every gate CI runs was also
      run locally first (`go test ./...`, `just lint`, `just arch-lint`,
      `just plimsoll`, `just coverage-check`, frontend tests + ESLint,
      markdownlint on both edited docs, and the full e2e suite)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1295
