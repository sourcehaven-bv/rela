---
id: IMPL-OS1Z5ZD
type: implementation-checklist
title: 'Implementation: Mobile: hamburger shows content through it and an empty Piles heading shows'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (Sidebar.piles.test.ts)
- [x] Integration tests written (test full flow, not just units) (mobile-layout.spec.ts runs the built SPA on rela-server at a phone viewport and reads the hamburger's computed background after scrolling)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a page with a sticky topbar keeps its layout; a user whose piles are all placed by `pile:` entries keeps the group and its +; the group comes in once the first pile exists)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new failure path)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: uses the existing pile helpers and SEED)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: none)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** On a 375x812 viewport, the entity page TKT-003
scrolled by 300px shows the hamburger on a white surface with a border over the
"priority" row; before the fix the text showed through it. The drawer for a user
without piles has no Piles heading. The full mobile e2e project and
piles.spec.ts and page-chrome.spec.ts pass.

## Quality

- [x] Code follows project patterns (check similar code) (the surface is a library option, as frontend/CLAUDE.md asks, not a global App.vue rule; the optional group follows declaredGroup in Sidebar.vue)
- [x] Checked for DRY opportunities
- [x] No security issues introduced (display only)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no new failure path)
- [x] No debug code left behind
