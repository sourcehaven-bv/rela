---
id: IMPL-YHQFMR
type: implementation-checklist
title: 'Implementation: Space Create menu on an entity page does not link the new item to the page entity'
started: "2026-10-05"
completed: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) (SpaceCreateMenu.test.ts mounts the menu against real page and space stores; TestPageScope_SidebarWire covers the served payload)
- [x] Happy path implemented
- [x] Edge cases from planning handled (off-page create, unmatched type, ambiguous relations, add another, faced anchor)
- [x] Error handling in place (errors surfaced, not swallowed) (a failed link reuses the "link it by hand" toast)

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

**Verification Evidence:** Sample project with a topic entity page (a relation
tab and a root timeline tab) and a space Create menu offering task, run on a
locally built rela-server with the new SPA.

- `_sidebar` serves `links: [{type: task, relation: contains, direction: outgoing}]` on both tabs.
- Create > Task on the timeline tab: the task was created and `TOPIC--contains--TASK` was written.
- Create > Task from the topics list (no entity page): the task was created with no relation.
- Mutation check: removing the link call fails both "links a new row" tests.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (the anchor link write is shared by the tab and the Create menu through one helper)
- [x] No security issues introduced (the link is an ordinary relation write under the normal ACL; links are derived from config, which is not secret)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
