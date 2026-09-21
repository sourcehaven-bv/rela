---
id: IMPL-DUPZ8K
type: implementation-checklist
title: 'Implementation: Duplicate an entity from the detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` against `prototypes/data-entry/project` as an editor
principal and drove the flow in a real browser.

- The Duplicate button renders in the desktop header row: the detail page for
  TKT-002 showed `["Edit E", "Duplicate", "Delete Del"]` (AC1).
- Clicking it opened the dialog titled "Duplicate Ticket" with two groups:
  Outgoing (`belongs to` ×2, `tagged` ×1, both checked) and Incoming
  (`blockedBy` ×4, unchecked). Counts, grouping and default selection all
  match AC2/AC3. Screenshot captured.
- Against the API directly, one batched create carrying BOTH an outgoing
  (`belongs-to`) and an inverse-keyed incoming (`blockedBy`) relation returned
  201 and read back with both edges present. This is the evidence that
  retired AC17 and RR-LOWNMB — the create body resolves both directions in a
  single call, so the post-create write path the plan proposed was unnecessary
  and would have introduced the non-atomicity it feared.

Automated coverage, all green:

- `internal/dataentryconfig`: 9 new cases (AC9, AC10, AC19c) — the allowlist,
  the unknown-property refusal, the explicitly-empty-list refusal, and a
  `duplicate:`-only entry loading without `detail_view`.
- `duplicatePrefill.test.ts`: 17 cases (AC15, AC16, AC10a, AC10b, AC19b, AC2,
  AC3, AC6, AC7, AC9).
- `DuplicateModal.test.ts`: 6 cases (AC10c, AC20, AC4, AC17 and the disclosure).
- `duplicate.spec.ts`: 2 e2e cases (AC5, AC6, AC7, AC8, AC21 and the incoming
  inverse-key round trip), asserted by reading the copy's relations back from
  the API rather than from the dialog.
- Full suites: 2925 frontend unit tests, 309 e2e, `internal/dataentry` +
  `internal/dataentryconfig` + `internal/apiwire/v1` Go tests.

Mutation-verified rather than assumed: disabling the state-machine exclusion,
the `file` exclusion, and the `provideInlineCreateDepth()` call each fails its
own test and nothing else.

**A real bug the e2e caught that unit tests could not.** The first e2e run
created the copy successfully but with no relations. Two independent causes,
both silent: (1) `relations.value` is not the payload for a `widget: cards`
relation — the submit path excludes those keys and reads `pendingCardChanges`
— and (2) `buildRelationsPatch` emits `entries` as the edge list while reading
`added` only to decide whether to emit the key, so populating `added` alone
sent an empty list. Each produced a 201 with missing edges and no error, which
is exactly the failure shape RR-VR2YGE predicted. Both are fixed and the
second is now pinned by a test in `relationsPatch.test.ts`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the prefill reuses `applyTemplate`
      rather than adding a second apply path, and enablement reuses
      `inline_create` rather than adding a `_duplicate` affordance key
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
