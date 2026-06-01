---
id: PLAN-DTMAB
type: planning-checklist
title: 'Planning: Enable contextcheck golangci-lint rule'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- Enable `contextcheck` in `.golangci.yml`, remove the commented-out block + explanatory comment (lines 16-19).
- Fix all 101 violations by threading the real request/caller `ctx` to the bottom of each call chain.

OUT of scope:
- Any unrelated linter or broader ctx-cancellation hardening beyond what contextcheck flags.
- Refactoring `bindingContext`/predicate internals beyond passing ctx through `passes`.

**Acceptance Criteria:**
1. `contextcheck` uncommented in `.golangci.yml`, 4-line comment gone. Test: grep config; `golangci-lint linters` shows it enabled.
2. `just lint` clean — zero contextcheck violations, zero new violations from other linters. Test: `just lint` exits 0.
3. No behavioral regression: every threaded ctx is the actual request/caller ctx, never a swapped-in `context.Background()`. Test: `just test` passes.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
- Built-in golangci-lint linter; no library needed.
- **Precedent in tree:** `App.outgoingRelationsCtx(ctx, id)` (services.go:77) is the ctx-aware twin of legacy `App.outgoingRelations(id)` (services.go:70) wrapping `context.Background()`. Migration shape: add `*Ctx` variant, route ctx-aware callers to it, retire background wrapper. RR-JWDHH confirms this direction.
- Predecessor **TKT-AHUNF** enabled the rest of the v2 linters and deferred contextcheck; this finishes it.
- dataentry HTTP handlers carry `r *http.Request` → `r.Context()` available at every call site.
- MCP prompt handlers receive `context.Context` but discard as `_` and create `context.Background()`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Two mechanical transforms, bottom-up, package by
package: A. `Non-inherited new context` (14×) — replace local
`context.Background()` with in-scope ctx (`r.Context()` / named first param).
MCP handlers: rename `_ context.Context`→`ctx`, drop the `Background()` line. B.
`should pass the context parameter` (86×) — add `ctx context.Context` first
param to each flagged helper, pass down to store/tracer/predicate call, update
callers.

Per-package: affordances/resolver.go (thread through
passes/applyFieldGrants/applyOptionGrants/metaFieldResults →
prog.Eval(ctx,...)); dataentry (add getEntityCtx; thread
getEntity/outgoingRelations/incomingRelations/listAllFromStore/computeDocumentHash/view+query
helpers); mcp (name+use ctx params; thread
buildStoreRelations/convertStoreEntity/tool helpers); analysis (collectEntities
chain + callers); scheduler/cli/validation/script/lua (thread available ctx).

**Alternatives:** nolint-suppress (rejected — leaves real gaps); `--new` only
(rejected — goal is full enablement); big-bang commit (discouraged — stage by
package).

**Files:** `.golangci.yml`, `internal/affordances/resolver.go`,
`internal/dataentry/*` (services, api_v1, handlers_api, commands, affordances,
actions, handlers_theme*, relations_modern, helpers, document), `internal/mcp/*`
(prompts, resources, convert, tools_*), `internal/analysis/analysis.go`,
`internal/scheduler/scheduler.go`, `internal/cli/validate.go`,
`internal/validation/validation.go`, `internal/script/executor.go`,
`internal/lua/*` + flagged `_test.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None changed. Pure ctx-threading refactor.

**Security-Sensitive Operations:** affordances resolver (`passes`→`prog.Eval`)
is on the auth path. Threading real ctx is strictly an improvement (eval becomes
cancellable); verdict logic untouched. Verify ACL/affordances tests pass — no
verdict drift.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 grep config + `golangci-lint linters`. AC2 `just lint`
exits 0. AC3 `just test` (race) for affordances/dataentry/mcp/analysis.

**Edge Cases:** helper called from both ctx-aware and ctx-less sites (thread ctx
into both); iterator store reads must carry ctx; test entrypoints may
legitimately originate Background — fix the helper signature, not by
suppressing.

**Negative Tests:** handler tests asserting 404/422 must pass unchanged — ctx
threading must not alter status codes.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** ripple into public signatures (mitigated: mostly unexported; exported
ones are internal-pkg, all callers in-tree, compile catches); Background-swap
silencing the linter without fixing (mitigated: review every Background()
deletion); large diff (mitigated: stage by package, lint+test per package).
Effort: **m**.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal refactor + lint-config change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (Skipped: user approved going straight to implementation; mechanical ctx threading with established in-tree precedent)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — design review skipped per user approval.
