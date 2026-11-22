---
id: IMPL-GNW481L
type: implementation-checklist
title: 'Implementation: Hide Add to pile on lists with list_buttons'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (chrome_test.go (validation, omit-when-unset), acl_audit_permissions_test.go, EntityList.piles.test.ts, SearchView.listButtons.test.ts)
- [x] Integration tests written (test full flow, not just units) (page_chrome_test.go TestListButtons_ResolvedPerPrincipal runs /_sidebar under a real acl.yaml; golden sidebar corpus regenerated)
- [x] Happy path implemented
- [x] Edge cases from planning handled (deletable rows keep selection without Add to pile; an older server without list_buttons shows the button)
- [x] Error handling in place (errors surfaced, not swallowed) (unknown keys under list_buttons fail config validation)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing test helpers (newTestAppV1, gatedNavPolicy, mountList))
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: none)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by the handler test and the component tests; e2e page-chrome covers the header_buttons twin)
- [x] Each acceptance criterion verified with test scenario from planning (hidden per principal, no checkboxes without a bulk action, validated and documented)
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code) (mirrors header_buttons)
- [x] Checked for DRY opportunities — repeated literals, expressions, or (listButtonsWire reuses ChromeToggle.Shown and permitsGatedUIElement)
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (a UX filter; pile endpoints keep their ACL; configedit keeps the key locked)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
