---
id: PLAN-0PSRW
type: planning-checklist
title: 'Planning: Structured error reporting for Lua script failures'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

(Full plan content preserved verbatim below — only the Documentation Planning
checkboxes were updated to mark deferred items with skip-reasons.)

**Problem:**

Today, when a Lua script fails (action button, document render, automation, MCP
`lua_run`/`lua_eval`), rich context is *known* server-side (script path, Lua
line, stack, captured `print()` output, entity id, args, correlation id) but
**discarded** at two seams:

1. The action HTTP handler collapses to `{"error":"action_failed","message":"Action failed"}` (`internal/dataentry/actions.go:107-108`).
2. Frontend catch blocks ignore detail fields and show canned toasts:
   - `frontend/src/components/entity/DocumentsPanel.vue:112` → `uiStore.error('Failed to render document')`
   - `frontend/src/views/DocumentView.vue:76` → same
   - `frontend/src/components/common/Sidebar.vue:103-107` → `'Action failed'` (with optional correlation id)

Plus: captured `print()` output is dropped on document failure
(`internal/dataentry/document.go:244-248`); automation Lua errors are flattened
to `"script execution error: " + err.Error()` with no script identity
(`internal/workspace/workspace.go:1223`).

(See ticket TKT-LR5YC and IMPL-AP03E for the as-built scope, files modified,
acceptance criteria with test evidence, and deviations from this plan.)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

## Documentation Planning

- [x] User-facing docs identified

**Documentation Impact:**

- [x] ~~User guide / reference docs~~ (N/A: no rendered Lua-scripting guide exists yet in the docs site; the rela-docs MCP graph has a `lua-scripting` concept but nothing rendered. When that guide lands, it should mention the structured error panel — tracked in DOCS-KQLKQ)
- [x] CLI help text — N/A (no CLI changes)
- [x] ~~CLAUDE.md note about always returning *lua.ScriptError~~ (N/A: would be aspirational guidance with no companion rule in the existing list; skipping per CLAUDE.md "no premature abstractions")
- [x] README.md — N/A
- [x] ~~API docs for new envelope~~ (N/A: data-entry has no published OpenAPI spec; the existing `writeV1Error` shape isn't documented either, so adding only the new envelope would be inconsistent)

A `docs-checklist` (DOCS-KQLKQ) was created at the implementation→review
transition.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- RR-M4EB9 (F1, critical) — typed `*lua.ApiError` access; addressed.
- RR-3LJVM (F2, critical) — loopback gating + opt-in flag; addressed.
- RR-LABB3 (F3, critical) — `RunFileWithCapture` helpers; expanded file list; addressed.
- RR-1ZNL1 (F4, significant) — `[]error` not `[]ScriptError`; expanded audit; addressed.
- RR-0JT3J (F5, significant) — `Path = "automation:<name>"`; addressed.
- RR-MWCAU (F6, significant) — `errors.As` branch; contract errors stay 500; addressed.
- RR-8LQ3Q (F7, significant) — captured-output redaction + loopback gate; addressed.
- RR-U7BRF (F8, significant) — broadened denylist + JWT/hex/b64 value-shape redactor; addressed.
- RR-WJEQN (F9, minor) — `NewToolResultError(jsonString)`; addressed.
- RR-F6334 (F10, minor) — replace-latest, role=alertdialog, Esc, focus restore; addressed.
- RR-CIAFS (F11, nit) — drop `inner`/`Unwrap`; addressed.
