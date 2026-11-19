---
id: PLAN-CN7479Q
type: planning-checklist
title: 'Planning: Root opens the space home; config for built-in sidebar rows'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the root route opens the current space's home; a `builtins:`
block (top level and per space) that hides or permission-gates the pinned
Search, Inbox, Flow tasks, Flow runs and Analysis rows. Out: the Validation
dashboard card (gap 2), entity page header buttons (gap 8), and any change to
the access of the pages behind the rows.

**Acceptance Criteria:**
1. `/` with spaces opens the home of the current or last used space, else the first permitted one (space store and router unit tests, Playwright spec).
2. `/` without spaces opens `/dashboard` (store unit test, Playwright spec).
3. `builtins:` hides or gates rows per space and at the top level (Go handler tests, Sidebar vitest, Playwright spec).
4. Unknown row names and malformed entries are load errors (dataentryconfig table test).
5. No block: every row shown as before (Go and vitest default cases).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small extension of existing spaces and nav permission patterns)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: config and UI wiring inside rela)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows rela's own nav `permission:` pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `permitsGatedUIElement` (internal/dataentry/views_handler.go) gates nav entries, spaces, page tabs and dashboard cards; reused for rows.
- `/s/:space` guard in frontend/src/router/index.ts already asks the server for a space home on a cold load; the root guard does the same.
- `resolveSpace` falls back to the first permitted space for an unknown or hidden id, so the remembered space is only a hint.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Config: `dataentryconfig.Builtins` map of row name to
`{hidden, permission}` on `Config` and `Space`, validated at load. Server:
`_sidebar` returns `hidden_builtins` for the resolved space (or top level),
computed with `permitsGatedUIElement`. SPA: Sidebar filters pinned rows and the
`/` shortcut; the root route guard redirects to `homeTarget(spaces, current)`,
with the space store remembering the last space in localStorage per base.

Alternatives: computing the hidden rows in the SPA from `_config` (rejected: the
SPA cannot evaluate permissions); letting a top-level block default every space
(rejected: mirrors `navigation:`, which is exclusive with `spaces:`).

**Files to modify:**
internal/dataentryconfig/{builtins.go,config.go,space.go,validate.go},
internal/apiwire/v1/responses.go, internal/dataentry/views_handler.go,
frontend/src/{router/index.ts,stores/space.ts,components/common/Sidebar.vue,types/config.ts},
e2e spec, docs-project guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** data-entry.yaml (operator): row names checked
against an allowlist at load. localStorage last space (browser): checked against
the space id pattern and only sent as `?space=`; the server resolves it against
permitted spaces.

**Security-Sensitive Operations:** None. Row gating is a UX filter like nav
`permission:`; the pages behind the rows keep their own ACL. Config is not a
secret.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:**
- Every row hidden: the pinned band is dropped.
- A space home of `/`: opens the space dashboard, no redirect loop.
- Stored last space no longer permitted: server falls back to the first.
- No acl.yaml: a gated row is shown.

**Negative Tests:** Unknown row, hidden and permission both set, neither set,
padded permission, top-level block with spaces.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** The root guard adds a sidebar request on a cold load at `/`; the same
as the existing `/s/:space` guard, and skipped once the store has loaded.
Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - via the docs-project spaces/navigation guide

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change on existing patterns; design reviewed together with the code by cranky-code-reviewer and rela-security-reviewer)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above; findings recorded as review-responses)

**Design Review Findings:** N/A
