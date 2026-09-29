---
id: PLAN-C2JBBQ
type: planning-checklist
title: 'Planning: seqtrace: sequence diagrams of traced request flows'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: a diagnostic tool under `tools/seqtrace` (rewriter, runtime, diagram
renderer, CLI), `just seqtrace-*` recipes, and a self-contained postgres + ACL
demo. Out: changes to any production package, linking the runtime into normal
builds, tracing code outside the module (stdlib, pgx, bleve), and linking
`store.Event` observers and durable-queue jobs to their origin.

**Acceptance Criteria:**

1. Rewritten files compile and keep line numbers and `//go:` directives
(`TestFile_KeepsLineNumbersAndDirectives`).
2. Parents are linked across goroutines via ctx, go and closure
(`TestDemo`).
3. Diagrams fold repeats, collapse value packages, and show argument and
result summaries (`TestRender`, `TestRenderValues`).
4. Summaries are bounded and redact secret-named parameters
(`TestSummarize`, `TestSummarizeArgs`).
5. `just seqtrace-demo` leaves the checkout unchanged and produces an index
plus one page per scenario (manual run: ten scenarios, all expected status
codes, all diagrams parse in Mermaid).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: diagnostic tool outside the product; options surveyed below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- go-callvis: static call graph, no ordering or numbering; rejected.
- runtime/trace and OpenTelemetry: need manual spans across the codebase;
rejected for a whole-codebase view.
- Source rewriting via `go build -overlay` needs no production change and no
runtime dependency in normal builds; chosen.
- Prior art in repo: `frontend/stress` (FEAT-2M4D) for diagnostics harness
shape, `prototypes/perf/project` + `rela dev seed` for demo data.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Text-offset insertions (not AST reprinting) add an Enter/Exit prologue to every
function and wrap function literals to capture their creator. Per-goroutine
stacks give parents; ctx values, go-statement spawns and closure creators link
goroutines. Values are summarized by reflection without calling `String`. The
renderer builds a call tree and emits Mermaid. Dependencies: stdlib only.

**Files to modify:**

`tools/seqtrace/**` (new), `justfile`, `.go-arch-lint.yml`, `.testcoverage.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Operator-supplied env vars and flags (paths, regexps; invalid regexps fail at
start). The trace file is read back by the same tool; malformed lines are
skipped.

**Security-Sensitive Operations:**

Trace files contain argument summaries. Secret-named parameters are redacted,
entity properties are never summarized, and the README says to treat traces like
logs. The demo binds postgres and the server to 127.0.0.1 and writes only under
`.ignored/`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:**

Unnamed, blank and unparenthesized results; generic receivers; cgo files;
directive functions; typed-nil errors; multi-byte truncation; a trace cut short
mid-line; orphaned parents.

**Negative Tests:**

Cgo and empty files are left alone; a truncated trace line is skipped.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

Rewriting may fail to compile on new syntax: `just seqtrace-demo` builds the
whole server and `TestDemo` builds a real program. Spawn matching by name can
misattribute concurrent spawns: documented in Limits.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

`tools/seqtrace/README.md` (new). No product docs change.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: implementation predates the ticket; `/code-review` in the review phase covers the design)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run, see above)

**Design Review Findings:** N/A
