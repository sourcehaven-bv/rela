---
id: REV-EVH5
type: review-checklist
title: 'Review: Migrate CLI to scoped services helper (drop package globals)'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] No critical findings
- [x] Significant findings addressed in-PR or deferred to TKT-2W0X with rationale
- [x] Minor findings addressed
- [x] Tests pass under `-race`
- [x] `just ci` passes end-to-end

**Summary:** see IMPL-UXTT for the full disposition table.

**Code Review Summary:**

Cranky review verdict was "ship it" with no critical issues. Real findings
addressed in-PR:

1. **Silent-nil accessor** — `cliXFromContext` now panics with a targeted
message instead of returning a nil that nil-derefs three frames deep.
2. **Duplicate `ResolveEntityType`** — deleted workspace's method; CLI uses
the lifted free function exclusively.
3. **testCmd assertion** — panics with "applySeeder must be called before
testCmd" if testCtx is nil, surfacing test setup bugs at their source.

Deferred to TKT-2W0X (already filed): dead-looking bundle methods, workspace
type leaks via `cliAnalyze` interface signatures. Those resolve naturally when
the facade methods lift into dedicated packages.

The cranky review unusually called out that the plan is "self-aware about its
transitional shapes" — the explicit "this is transitional, here's the exit
ticket" pattern was load-bearing in turning a potential service-locator critique
into an accepted middle ground.
