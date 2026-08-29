---
id: PLAN-6EG1PJ
type: planning-checklist
title: 'Planning: Extract markdown AST helpers off lua.Runtime (plimsoll ratchet 105 → ~60)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: `internal/lua/markdown.go` only — receiver change for the 45
unexported methods currently on `*Runtime` (42 state-free AST methods →
`mdHelpers`; 3 graph-coupled entity-refs methods → `mdEntityRefs`);
`registerMarkdownModule` stays on Runtime and constructs both. Lower
`//plimsoll:max-methods` in `runtime.go` from 105 to 60. OUT: HTTP/cache/AI
binding clusters (follow-up tickets), any behavior change, any exported-API
change, any Lua-visible change.

**Acceptance Criteria:**
1. `just plimsoll` passes with Runtime's directive at 60 (ratchet, not bump).
2. `go test ./internal/lua/...` and full `go test ./...` pass unchanged — the
markdown test suite exercises every binding through Lua, so a green run IS the
behavior-preservation proof.
3. `just arch-lint` clean.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: mechanical refactor with an in-package precedent)
- [x] ~~Searched for existing libraries~~ (N/A: no new functionality)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — codebase survey performed instead; findings below.

**Existing Solutions:**
- `urlHelpers` (internal/lua/urls.go:26) — the canonical in-package template;
its godoc argues exactly this case (pure function group hung off Runtime).
- `registerCryptoModule` free function (crypto.go:40) and
`registerDateHelpers(ls, rela)` (date.go:30) — free-function variants kept off
Runtime explicitly citing the plimsoll line.
- `deps.go` `ReadDeps`/`WriteDeps` + `EntityReader` — the narrow-deps pattern
`mdEntityRefs` follows.
- `FlowRuntime` (flow.go) — sibling runtime type precedent.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Verified by grep: of the 46 `*Runtime` methods in
markdown.go, 42 touch no Runtime state except `r.L` (only
`NewTable`/`SetField`/`NewFunction`); the entity-refs trio additionally touches
`deps.Meta`, `deps.VisibleReader` (via `Runtime.reader`) and `callerCtx`. So:
- `type mdHelpers struct { ls *lua.LState }` — receiver change in place for the
42 methods; `r.L` → `m.ls`.
- `type mdEntityRefs struct { meta *metamodel.Metamodel; reader EntityReader;
ctx func() context.Context }` — the trio moves here; nil-reader/nil-meta raise
the exact same Lua errors as today (`Runtime.reader`'s message inlined). `ctx`
stays a closure over `Runtime.callerCtx` so timeout-derived context changes keep
propagating.
- `registerMarkdownModule` (stays on Runtime) constructs both and binds their
methods; wiring seam unchanged, mirroring `registerURLModule`.

Alternative rejected: converting all 42 to free functions threading `ls` through
~20 internal helpers — same result, strictly more churn, and the struct form
matches `urlHelpers`.

**Files to modify:**
- internal/lua/markdown.go (receiver changes + two type decls)
- internal/lua/runtime.go (directive 105 → 60)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Unchanged — Lua script inputs validated by the
same binding code, moved verbatim.

**Security-Sensitive Operations:** One load-bearing invariant to preserve:
`luaMdEntityRefs` must keep reading through the ACL-gated `deps.VisibleReader`
with `callerCtx()` (NOT `context.Background()`) — the RR-ZA452J comment moves
with the code. `mdEntityRefs.reader` is typed as the same `EntityReader`
consumer interface and is handed `deps.VisibleReader` at the single wiring
point; no new read path is introduced.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: no new behavior — existing Lua-level suite is the integration test)

**Test Scenarios:** Existing `markdown_test.go` (plus document/script suites)
drive every binding through actual Lua scripts; they compile against the package
internals and pass unchanged.

**Edge Cases:** Receiver identity: `m.ls` must be `r.L` (same LState the
bindings are registered on) — guaranteed by constructing `mdHelpers{ls: r.L}` in
`registerMarkdownModule`.

**Negative Tests:** Existing tests for nil-reader raise and unknown-type errors
in entity_refs continue to pass with identical messages.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low — unexported-only moves, zero call sites outside the package.
Main risk is a missed `r.` reference during the mechanical edit; the compiler
catches it. Effort: m (large diff, small decision surface).

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor, no user-facing change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor kind)

**Documentation Impact:** N/A — internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: mechanical extraction following the documented in-package precedent (urlHelpers godoc); no new interfaces or behavior — design decided and recorded in TKT-N0IKN9's approach section)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no review run)

**Design Review Findings:** N/A
