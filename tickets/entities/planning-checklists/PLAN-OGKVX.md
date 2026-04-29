---
id: PLAN-OGKVX
type: planning-checklist
title: 'Planning: Show Lua error details for data-entry action failures'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** When a list-action runs over multiple selected entities and the Lua
script fails on one or more of them, the user sees only a counts-based error
toast. The structured `ScriptError` envelopes — which contain `script.path`,
`lua.line`, `lua.message`, source snippet, stack frames, and correlation IDs —
are received from the API but dropped on the floor by
`useListActions.executeAction`.

**Already in place (do not rebuild):** Backend produces the envelope; frontend
has `ScriptErrorDialog`/`ScriptErrorPanel`/`useScriptErrorStore`/`isScriptError`
guard, mounted in `App.vue`. Sidebar single-action flow already wired. Axios
interceptor surfaces the envelope as the rejection reason.

**Scope (IN):** Update `useListActions.executeAction` to inspect rejections,
dispatch first `ScriptError` to the dialog, capture `triggerEl` for focus
restore. Tests cover all branches.

**Scope (OUT):** Backend changes (already there); new components/stores;
multi-error timeline; bulk_set error handling; sidebar (already wired).

**Acceptance Criteria:**

1. List-action script failure → `ScriptErrorDialog` opens with envelope.
2. Summary toast continues to show `N failed, M succeeded`.
3. Multiple failures → first envelope only (v1 limitation).
4. Non-script rejections → no dialog, only summary toast.
5. Keyboard focus restored to a sensible element after dismiss.
6. No regression in successful list-action flow.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:** TKT-LR5YC (commit `055961cc`) added structured
`ScriptError` envelopes across all Lua surfaces and the frontend infrastructure.
Sidebar single-action flow at `Sidebar.vue:108-119` is the reference wiring to
mirror. Axios interceptor at `client.ts:36-44` re-rejects `script_error`
envelopes as the rejection reason directly, so `Promise.allSettled` rejections
carry the envelope unwrapped.

**Conclusion:** Single-file edit to `useListActions.ts`, plus tests.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `useListActions.executeAction`: after `Promise.allSettled`, scan
rejections for the first matching `isScriptError` and call
`scriptErrorStore.show(...)`. Keep the existing summary toast.
- Plumb `triggerEl?: HTMLElement | null` through `triggerAction` →
`executeAction`, and through `onRequestConfirm` to support the confirm-modal
flow.
- Show first script error only (v1).

**Files modified:**

- `frontend/src/composables/useListActions.ts`
- `frontend/src/components/lists/EntityList.vue`
- `frontend/src/stores/scriptError.ts` (detach guard for focus restore)
- New: `frontend/src/composables/useListActions.test.ts`
- New regression: `frontend/src/stores/scriptError.test.ts`

**Alternatives considered:**

- Show all script errors stacked: rejected (UI is single-error).
- Server-side parse of error string into structured fields: rejected
(envelope already provides structure).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `ScriptError` envelope is produced by trusted
backend, structurally validated client-side via `isScriptError`. Rendered with
text interpolation in `<pre>` (no `v-html`) — XSS-safe. Loopback-gating of
`source`/`stack`/`captured_output` enforced server-side; frontend untouched.

**Security-Sensitive Operations:** None new. Same data already exposed via the
sidebar single-action flow.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** unit tests cover ACs 1–4 and 6; AC5 covered by the new
detach-guard regression test in `scriptError.test.ts`. Manual e2e deferred —
sample project has only `set:` actions; unit-test coverage substitutes.

**Edge Cases:** mixed rejections; all-non-script rejections; many script-error
rejections; confirm-modal flow; trigger-element detached after row removal.

**Negative Tests:** non-`ScriptError` rejection reason, empty selection.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** trigger element detachment (mitigated by `document.contains` guard in
`dismiss`); first-only error UX surprise (toast still shows count); extending
optional-param composable API (backward compatible).

**Effort:** xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~User guide / reference docs~~ (N/A: TKT-LR5YC docs already describe ScriptErrorDialog UX; this ticket extends it)
- [x] ~~CLI help text~~ (N/A: no CLI change)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns)
- [x] ~~README.md~~ (N/A: no project-level change)
- [x] ~~API docs~~ (N/A: no API change)
- [x] N/A — internal wiring fix; user-facing behaviour documented by existing dialog.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: xs-effort wiring fix, no architectural decisions; cranky-code-review run during review phase covered design concerns)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run; code-review findings tracked as review-responses)

**Design Review Findings:** N/A — see code review (`/code-review`) findings in
the linked review-response entities (RR-7HDS3, RR-1EY20, RR-1CE17, RR-OGRML,
RR-0RLYP, RR-YXX7C, RR-CDK7C addressed; RR-CJ41O, RR-VZA4X, RR-THTVI marked
won't-fix/deferred with reasons).
