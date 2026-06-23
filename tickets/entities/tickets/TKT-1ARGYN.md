---
id: TKT-1ARGYN
type: ticket
title: 'Frontend: surface autosave validation warnings + 422 errors per-field (A7)'
kind: enhancement
priority: medium
effort: m
status: ready
---

Re-scoped A7 (was "surface autosave validation warnings"). Model-agnostic
frontend half of FEAT-13863O / DEC-OYV2AK — renders the severity the server
sends, forward-compatible with strict mode without depending on it.

**Warnings (amber, non-blocking) — needed in all modes**
- `useAutoSave` already routes server 200+warnings into `fieldWarnings` / `contentWarning` / `relationWarnings` (computeds, exported) but NOTHING renders them — the soft-validation contract is dark on the main edit path.
- LIVE BUG: `categorizeWarnings` early-returns on an empty warnings array (core path src/composables/useAutoSave.ts:454), so a clean follow-up save never clears a stale warning. Fix: reset the relevant channel's warnings each successful save.
- FieldShell gains a `warning?: string` prop rendered amber (no `has-error` red border); FieldRenderer passes it through; DynamicForm binds `autoSave.fieldWarnings[prop]`. Content warning under MarkdownEditor; relation warnings on the RelationCards/RelationPicker widget headers via the existing widgetId keys.
- Precedence: client/server error wins the prominent slot; warning shows only when no error.

**Errors (red, blocking) — makes the SPA strict-mode-ready**
- A 422 already flows through ApiError.validationErrors (PR #960). Bind it to the per-field error slot so a strict-mode 422 shows as a blocking per-field error and prevents the green saved-state on the autosave path (a 422 means the keystroke did NOT persist).
- RR-K4AU69 reversal: KEEP and WIRE DynamicForm's `validationErrors` branch (earlier deferred as 'dead'). It becomes live under strict mode — ProblemDetail.errors[] IS populated when strict 422s.

Worth running the app to tune the amber visual rather than shipping on tests
alone. Independent of the Colada migration arc (touches forms, not lists).
