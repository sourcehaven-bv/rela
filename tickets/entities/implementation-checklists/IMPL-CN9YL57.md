---
id: IMPL-CN9YL57
type: implementation-checklist
title: 'Implementation: Link style, phone layout, sidebar title and empty list text'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (UrlWidget link style and name; DashboardView labels and roles; dashboardCards isShortToken and nowrap; SortMenu icon slot; EntityList.empty.test.ts with empty_message; TestToV1EntityType_LabelPlural; TestList_EmptyMessage; screenEdits.test.ts)
- [x] Integration tests written (test full flow, not just units) (e2e mobile-layout: a list keeps its search box, leaves no empty band and casts no drawer shadow; the golden wire corpus carries label_plural)
- [x] Happy path implemented
- [x] Edge cases from planning handled (blank label_plural counts as none; empty dashboard cells stay in the accessibility tree; tools wrap when they do not fit one phone row; long short-tokens widen the value column instead of overflowing)
- [x] Error handling in place (errors surfaced, not swallowed) (no new error paths; a missing label falls back to a neutral text)

## Test Quality

- [x] Using fixture builders or factories for test data (existing mount helpers and schema store fixtures)
- [x] No hardcoded values in assertions when object is in scope (the expected sentences are literal on purpose: they pin the user-facing text)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (360px list with filter controls; empty dashboard cells; a type without label_plural)

**Verification Evidence:**
Served a copy of the harbourmaster project with this branch and checked it in a browser at 360px, 375px and desktop width. At 375px the list shows search, Sort and Filter in one row, with no empty band under the header and no shadow strip. At 360px with filter controls the tools wrap to two rows and nothing overflows (document and tools scrollWidth equal their client width). Dashboard table cards stack as label and value pairs, no card scrolls sideways, and empty cells are 1px high. The menu button shows no box, and the long workspace name wraps with the close button beside it. With label_plural set the empty list reads "No People found."; without it, "Nothing found.". At desktop width the Download PDF links use the accent colour, and their accessible name is "Download PDF" with no title. Focused vitest (745 tests), vue-tsc, eslint, the Go tests for openapi, apiwire and dataentry golden parity, and the e2e specs mobile-layout, dashboard-card-rendering and url-link pass.

## Quality

- [x] Code follows project patterns (check similar code) (source-reading style tests as in focusRing.test.ts; the visually hidden pattern; schema store computed getters)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (label_plural is schema metadata already readable by every client; text goes through Vue interpolation)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
