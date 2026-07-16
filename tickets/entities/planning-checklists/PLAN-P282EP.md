---
id: PLAN-P282EP
type: planning-checklist
title: 'Planning: rela acl who-can <verb> <entity> — list principals with access to one entity + provenance (UC3)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: `rela acl who-can <verb> <entity>` on ONE entity, all four verbs,
all-routes terminal-fact provenance, text+JSON, everyone-once, existence gate.
OUT (deferred): full hop-chains, `can`/`map`, type-aggregation, drift,
conformance-assertions, web view. See [[TKT-9089I6]].

**Acceptance Criteria:** documented in the ticket body (11 criteria) each mapped
to a test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** [[RES-8TX9KF]] (surveyed 8 use-cases, ACL model, provenance
decision).

**Existing Solutions:** Reused the acl resolver (`Request.ForEntity`,
`PermitsRead`, `Source` taxonomy) — no library needed; the feature is composing
existing evaluation primitives. `internal/aclaudit` is the sibling
policy-linter; who-can is the reserved Tier C/D territory.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** new `internal/aclmap` engine taking narrow consumer-side
interfaces; new `acl.Request.AccessRoutes` unifying read (via `PermitsRead`) and
write (via `grantsVerb`) provenance; CLI command under `rela acl`. Rejected:
reconstructing read from `computeForEntity` (false-negative risk — see
[[RR-7UXWNA]]).

**Files to modify:** `internal/acl/access.go` (new), `internal/aclmap/*` (new),
`internal/cli/acl_whocan.go` (new), `internal/cli/acl.go`, `.go-arch-lint.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** verb (Kong enum allowlist + `Verb.Valid()`
fail-closed guard); entity ID (existence-gated via `store.GetEntity`); acl.yaml
(validated against metamodel). This is a read-only reporting tool — no writes.

**Security-Sensitive Operations:** the whole feature IS access reporting; the
load-bearing invariant is no read false-negative — guarded by the
read-vs-runtime conformance test.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** all six route kinds, multi-route redundancy, everyone-once,
read-vs-runtime conformance, missing-entity error, unknown-verb error, JSON
schema, plus CLI-level text/JSON/missing/no-policy tests.

**Edge Cases:** non-existent entity (errors), `everyone: read:["*"]` (global
once), group entities (excluded as non-actors), blank assignment key (skipped),
unknown verb (fail-closed).

**Negative Tests:** missing entity → error; unknown verb → error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** read/write path drift (mitigated: read routed through real
`PermitsRead` + conformance test); O(principals) cost (bounded by depthCap=5,
noted, reverse-index deferred). Effort: M.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** the data-entry-transport caveat is documented in
`--help` and the command godoc. A `docs/cli-reference.md` update is a candidate
for the follow-up `map`/`can` slice when the full command family lands;
~~standalone docs page~~ (N/A: single subcommand, self-documenting via
`--help`).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** [[RR-7UXWNA]] (critical), [[RR-N16SDV]],
[[RR-CY6WYR]] (significant), [[RR-GC751G]], [[RR-K72ML0]] (minor) — all
addressed, plan revised before implementation.
