---
id: REV-3Q2DKJ
type: review-checklist
title: 'Review: A required file property makes the create form unsubmittable: Save silently does nothing'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Frontend unit tests — 2055/2055 across 126 files
- [x] E2E — 272 passed / 8 pre-existing skips / 0 failures
- [x] `npm run typecheck` and e2e `npx tsc --noEmit` — clean
- [x] `npm run lint` — 0 errors; `npx eslint` on changed e2e files — clean
- [x] `just arch-lint` — OK
- [x] `just docs-check` — in sync (no doc change needed; behaviour now matches
      what `docs/data-entry.md` already implied)

## Code Review

- [x] Self-review of the diff
- [x] No new findings requiring `review-response` entities

The fix is three lines of behaviour in one function plus an error-clearing
mirror of the existing `updateField` pattern. Reviewed for: the gate not leaking
into edit mode (it reads `stagedFiles`, which only create mode populates); no
interaction with the wizard prune (that filters uploads, this filters
validation); and no change to the server contract.

Two mistakes of mine were caught during verification and fixed:

- The first fixture put `required: true` on the **shared** `bug` type, which
  made every other test's bug emit a `required_property_unset` warning and broke
  the oversize-upload assertion. Moved to a dedicated `signoff` type.
- `submitExpectingNoCreate` counted the create-mode **dry-run** POST (same path,
  `?dry_run=true`) as a create, so it failed against correct code. Excluded.

Also corrected a claim in the bug's own writeup: I had said the file widget
renders no error slot. It does — `FieldShell.vue:46` renders `.field-error` for
every field type. The defect was that the message was unactionable, not absent.

## Verification

Manual run against a real `rela-server` with `evidence: {type: file, required: true}`:

```
1. creates after submit with no file (want 0): 0
2. still on the form (want true): true
3. visible error: ["This field is required"]
4. error after staging (want []): []
5. creates after staging (want 1): 1
6. attached: ["req-evidence.txt"]
7. property stamped: "attachments/REP-001/evidence/req-evidence.txt"
```

Regression tests added:

- Unit (`DynamicForm.attachments.test.ts`): submit blocked with the field
  flagged; a staged file satisfies the requirement and create proceeds; the
  standing error clears on staging.
- E2E (`attachments-create.spec.ts`): blocked Create issues no POST and shows
  the required error, then staging a file lets the create through and the
  attachment is readable back from the server.

Each fails against the pre-fix code — the unit reproduction was
`created: 0, uploaded: 0` before, `1` and `1` after.

## Acceptance

| Criterion | Result |
|-----------|--------|
| Create blocked while a required file is unstaged | **PASS** (manual 1-2, unit, e2e) |
| The reason is visible to the user | **PASS** (manual 3) |
| Staging a file clears the error | **PASS** (manual 4, unit) |
| Staging a file unblocks Create, and the file attaches | **PASS** (manual 5-7, unit, e2e) |
| No regression to non-required file properties | **PASS** (existing 7 attachment e2e + full suite green) |
| No change to server behaviour | **PASS** (no Go changes; `required` stays soft) |
