---
id: IMPL-O7OQTL0
type: implementation-checklist
title: 'Implementation: World switcher: labels and opt-in placement; hidden worlds; per-record toggle'
started: "2026-11-07"
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

Ran a server built from this branch against a copy of
`prototypes/worlds/project` with `label:` on `published` and `editorial`,
`hidden: true` on `site-nl`, and the ACL removed. Checked in the browser:

- AC1/2: the sidebar shows "Gepubliceerd | Redactie" as links; `site-nl`
  (no label) shows its name.
- AC3: `site-nl` is absent on `/`; on `GUIDE-001@nl?world=site-nl` it is
  listed and marked current.
- AC5: `app.world_switcher: none` removes the switcher; the face toggle
  stays. `/_config` serves `header` when the key is unset.
- AC6: two and three segments render as links; vitest covers the select for
  four or more.
- AC7: on `POL-001@draft` the toggle marks Draft; Published opens
  `POL-001@published` with no `?world=` (the default world); Draft then
  opens `POL-001@draft?world=editorial` and the sidebar marks Redactie. At
  640px the toggle stays in the header and the "⋯" menu holds only Publish
  and Dupliceren.
- With three segments in the sidebar, the longest label is truncated with an
  ellipsis.

Tests: `go test` for metamodel, mcp, dataentryconfig, configedit, apiwire,
dataentry (config and worlds); `npm run test:run` (396 files, 5985 tests);
`npm run typecheck`; eslint on the changed files; e2e `faces-*` specs.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
