---
id: PLAN-LI54SS
type: planning-checklist
title: 'Planning: Entity header: one Convert menu and overflow for actions that do not fit'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

In scope: the desktop action row of the entity detail header. Out of scope: the
app shell's RlPageHeader, which this page does not use.

## Research

- [x] Checked codebase for similar patterns or reusable code
- [x] Reviewed relevant rela concepts for prior art

CopyMenu (one offer is a button, several a menu), the entityActions list and
EntityActionsMenu that the phone layout already uses.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

A ResizeObserver measures the full row against the header width minus the
title's CSS min-width. A container query was rejected: it can hide items but
cannot depend on the row's own content width.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Security-sensitive operations identified (file access, auth, crypto)

Layout only. The overflow menu draws the same gated action list.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Integration test approach defined (not just unit tests)

Unit tests for the menu, the fit composable and the header. A story with a
browser play test for button labels. A browser check at 1600, 1100 and 800px on
the atlas config.
