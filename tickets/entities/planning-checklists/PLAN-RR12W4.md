---
id: PLAN-RR12W4
type: planning-checklist
title: 'Planning: visibility: new internal/visibility package — Reader (PolicyReader/AllowAllReader) + tracer decorator + conformance suite'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: new `internal/visibility` package only — `Reader` interface + `PolicyReader`
+ `AllowAllReader`, a `tracer.Tracer` visibility decorator, conformance test
suite, arch-lint component block. Nothing rewired: no consumer
(dataentry/lua/scheduler) changes in this PR.

OUT: export fix (TKT-L9Q669), Lua/scheduler wiring (TKT-ZF2DTV), MCP, CLI, any
change to `internal/tracer`/`internal/store` themselves, per-job role authoring.

**Acceptance Criteria:**

1. `Get` on an ACL-hidden entity returns `(nil, false, nil)` — byte-identical to a genuinely missing id (RR-NGMI invariant; no existence oracle).
2. **`Get` enforces stored type == claimed type post-load** (RR-SRZK6X): an id whose stored entity is of a different type than the caller's claim returns `(nil, false, nil)` — the type-mismatch check lives IN the package, never delegated to consumers (BUG-ZWTDH9 read-side analog).
3. `Get` on a visible entity with a field-visibility policy returns a **copy** with `Visible[name]==false` properties absent; the stored entity is unmutated.
4. When the type's display property is hidden, `DisplayTitle` derivations over the redacted copy resolve to the entity ID (stripping the property suffices — the copy has no precomputed title channel).
5. `Filter` preserves order, returns a fresh slice, batches `PermitsReadMany` per type, drops a whole type fail-closed (loud log) on gate error.
6. **`FilterRelations`** (RR-Y7P4MQ): a relation is visible iff **both endpoints are visible** (FROM ∧ TO — the relation-history precedent); hidden-endpoint relations dropped, order preserved, endpoint visibility resolved via one batched gate pass.
7. Tracer decorator: hidden node prunes its entire subtree in `TraceFrom`/`TraceTo`; `FindPath` through a hidden intermediate returns nil exactly like no-path; `FindOrphans` drops hidden ids (batched by type); `HasCycle` with a hidden start behaves as with a nonexistent start.
8. **Tracer field-redaction is alias-safe and title-complete** (RR-6IL3X7, RR-5N4K35): visible nodes get a freshly built filtered `Properties` map (the store's aliased map is NEVER mutated — pinned by a store-unmutated test), and `TraceResult.Title`/`PathStep.Title` fall back to the ID when the title property is hidden for that node.
9. `AllowAllReader` and the nop-gated path are pass-through: outputs deep-equal to raw store/tracer access (NopACL parity).
10. Constructors reject nil required collaborators.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-PSZZKU (done); decision DEC-ZBI39P (accepted).

**Existing Solutions:**

- `search.VisibleSearcher` + `storetest.RunVisibleSearchTests` — the in-repo precedent for "ungated base + visibility wrapper + conformance suite" (TKT-BA8BSX). This package generalizes it.
- Row-gate engine: `acl.Request.PermitsRead/PermitsReadMany` (`internal/acl/request.go`), ctx-amortized via `acl.FromContext`/`WithRequest`.
- Field-verdict engine: `affordances.PolicyResolver.FieldVerdicts` (`internal/affordances/resolver.go:351`), `Visible` sparse map (absent=visible, false=hidden).
- Shapes being hoisted: `dataentry.visibleReader.getVisible/filterVisible` (gate-first ordering, fail-closed type-drop), `affordanceService.copyVisibleProperties` (fresh-map redaction).
- Relation gating rule: relation-history's both-endpoints (FROM ∧ TO) read gate (CLAUDE.md relation-versioning).
- Terminal-set filtering precedent for tracer output: `visibleAnalysisIssues` (TKT-QU7REX).
- No external library applies — this is policy composition over in-repo engines.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

The package defines **narrow consumer-side interfaces** and composes them;
concrete adapters over acl/affordances live in the same package as convenience
constructors (one place, no drift).

```go
type RowGate interface {
    PermitsRead(ctx, entityType, id string) (bool, error)
    PermitsReadMany(ctx, entityType string, ids []string) (map[string]bool, error)
}
type FieldRedactor interface {
    // HiddenProperties reports the property names hidden from the ctx
    // principal for e. FAIL-CLOSED CONTRACT (RR-FJUQSF): an impl that cannot
    // compute verdicts must return the hide-everything set, never nil.
    HiddenProperties(ctx, e *entity.Entity) map[string]struct{}
}
type EntityGetter interface { GetEntity(ctx, id string) (*entity.Entity, error) }

type Reader interface {
    Get(ctx, entityType, id string) (*entity.Entity, bool, error)
    Filter(ctx, candidates []*entity.Entity) []*entity.Entity
    FilterRelations(ctx, rels []*entity.Relation) []*entity.Relation // both-endpoints rule
}
```

- `PolicyReader{gate, redact, get}`:
  - `Get` = gate FIRST → store read → **stored-type check (`e.Type == entityType`, else not-found — RR-SRZK6X)** → copy with hidden props stripped. Copy is shallow-per-property (values alias; read-out contract, documented).
  - `Filter` = hoisted `filterVisible` semantics verbatim (batch by type, fail-closed drop + `slog.Warn`, order-preserving fresh slice).
  - `FilterRelations` = collect endpoint ids → load types (one `GetEntity` per distinct id) → one `PermitsReadMany` per distinct type → keep a relation only when FROM ∧ TO are both visible (RR-Y7P4MQ). Relations have no field-level redaction today; row-gating is the whole contract.
- **No explicit title-fallback needed on an entity copy**: a redacted `entity.Entity` has no precomputed title channel; `DisplayTitle` recomputes from properties and falls back to ID. Conformance test pins it.
- `AllowAllReader{}`: pass-through, no copy (documented read-only contract, same as raw reads today).
- **Tracer decorator** `VisibleTracer{base tracer.Tracer, gate RowGate, redact FieldRedactor, get EntityGetter}` implementing `tracer.Tracer`, post-hoc filtering (base stays pure):
  - `TraceFrom/TraceTo`: collect all node (type,id) pairs from the returned tree, ONE `PermitsReadMany` per distinct type, prune hidden node + whole subtree. For each surviving node: **replace `Properties` with a freshly built filtered map — never `delete()` on the aliased store map (RR-6IL3X7)** — and **apply the title fallback (ID) when the node's `title` property is hidden (RR-5N4K35)**.
  - `FindPath`: gate every step (steps carry `Type`); any hidden step → nil (indistinguishable from no-path); apply the title fallback on surviving steps' `.Title` (RR-5N4K35).
  - `FindOrphans`: load entities for ids (needed for type anyway), **group by type, one `PermitsReadMany` per distinct type (RR-MYLUSZ)**; hidden or vanished ids dropped fail-closed.
  - `HasCycle(startID)`: resolve start type via getter, gate; hidden start → same result as nonexistent start.
- Constructors `NewPolicyReader`/`NewVisibleTracer` reject nil collaborators. Convenience constructors adapt `*acl.Declarative` + `*affordances.PolicyResolver`.
- Conformance suite exported as `visibilitytest.RunReaderTests` / `RunTracerTests` (mirroring `storetest`), reused by PR 2/3 wirings.
- Godoc records accepted residual: `FindPath` withhold-vs-no-path timing difference (RR-7V9XN7) — impractical to exploit on in-memory traversal, not worth constant-time engineering.

**Alternatives rejected** (RES-PSZZKU / DEC-ZBI39P): redaction in `store.Store`
(layering, backends, write path); ACL-aware tracer (violates pure-reader,
topology leak); per-consumer adapters (recreates by-convention drift);
consumer-side stored-type checks (BUG-ZWTDH9 class — rejected by RR-SRZK6X).

**Files to modify:**

- NEW `internal/visibility/visibility.go` (interfaces + doc), `policyreader.go`, `allowall.go`, `tracer.go`, adapters file, tests
- NEW `internal/visibility/visibilitytest/` conformance suite
- `.go-arch-lint.yml` — new `visibility` component: `mayDependOn: acl, affordances, entity, store, tracer` (`tracer` needed for the decorator's interface/types; no cycle: tracer imports only store)
- `.testcoverage.yml` — floor for the new package

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Inputs are internal: ctx principal (stamped by entry points), acl.yaml policy (validated at load by acl/affordances), entity data. No new external input surface; no parsing.
- Verdicts are allowlist-shaped by construction: `PermitsRead` answers affirmative permission; `Visible` sparse-map default follows the engine's own semantics.

**Security-Sensitive Operations:**

- The package IS the security control. Invariants enforced by tests: gate-BEFORE-read ordering (no existence oracle); **stored-type-equals-claimed-type post-load (RR-SRZK6X)**; fail-closed on gate error (hide, never reveal); hidden-path indistinguishability; redaction-on-copy (never expose or mutate the stored entity — incl. the tracer's aliased maps, RR-6IL3X7); title never leaks past redaction (RR-5N4K35); both-endpoints relation rule (RR-Y7P4MQ); unstamped principal under a real policy → deny, never fall open; `FieldRedactor` fail-closed contract (RR-FJUQSF).
- Error messages must not embed hidden property names/values (attribution stays in `FieldVerdicts.Attribution` for audit).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (conformance suite, table-driven, over memstore +
declarative ACL fixtures)

- AC1: same-bytes assertion for hidden-vs-missing `Get` results.
- AC2: entity of type B fetched via `Get(ctx, "A", id)` where policy allows A but denies B → `(nil,false,nil)`, identical to a miss.
- AC3: redacted copy lacks hidden prop; store entity deep-equal before/after.
- AC4: policy hides display property → `DisplayTitle` == ID.
- AC5: mixed-visibility candidates → order-preserved subset; erroring gate stub → type absent + log captured.
- AC6: relations where FROM hidden / TO hidden / both visible → only both-visible survive; endpoint gating batched (probe-count assertion on a counting gate stub).
- AC7: diamond graph A→B→C, A→D; hide B → trace from A shows D, not B nor C; `FindPath(A,C)` == `FindPath(A, nonexistent)`; orphans/HasCycle per criteria.
- AC8: visible node with hidden field → fresh filtered Properties map, store map unmutated (deep-equal store entity after trace); visible node with hidden `title` → `.Title` == ID in both TraceResult and PathStep.
- AC9: AllowAllReader + nop-gate PolicyReader vs raw store/tracer: deep-equal outputs.
- AC10: nil-collaborator constructor errors.

**Edge Cases:**

- Empty candidates → nil (pin `filterVisible` behavior); duplicate ids in Filter; relation whose endpoint entity no longer exists (drop fail-closed); entity with zero properties; policy hiding ALL properties (empty-props copy, not nil); trace root itself hidden (whole result nil, same shape as base tracer's unknown-id — pin base shape first); cycle through a hidden node (prune still terminates); `FindOrphans` id vanished between scan and gate (skip fail-closed).
- Concurrency: PolicyReader stateless per call; `acl.Request` is NOT goroutine-safe — ctx-attached Requests follow the existing per-request contract; `-race` parallel Get/Filter smoke over independent ctxs.

**Negative Tests:**

- Gate error on `Get` → error surfaced, no entity returned.
- Unstamped principal + real policy → deny (not open).
- Redactor stub returning hide-everything → empty-props copy, no panic (RR-FJUQSF contract case).
- A future impl violating gate-first ordering or the stored-type check fails the suite (that's the suite's purpose).

**Integration:** PR 1 has no consumer wiring by design; "integration" = the
exported conformance suite run against real memstore + real `acl.Declarative` +
real `affordances.PolicyResolver` (not stubs) — the same suite PR 2/3 re-run
against their wirings.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Per-node gating cost on large traces → batch `PermitsReadMany` per distinct type over the whole tree (mirrors `filterVisible`).
- `FindOrphans`/`FilterRelations` need entity loads to learn types → bounded by result size; batched gate probes (RR-MYLUSZ); noted in godoc.
- Semantic drift vs dataentry's `visibleReader` when PR 2 swaps it in → hoist semantics verbatim + pin with the suite now; note `Get` is STRICTER (in-package stored-type check) — PR 2 removes the now-redundant caller-side checks rather than double-checking.
- `AllowAllReader` returning uncopied entities while `PolicyReader` copies → aliasing asymmetry; documented read-only contract. Effort: m (as ticketed).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: internal package, no user-facing behavior in PR 1; docs-checklist decision deferred to PR 2/3 where behavior changes land)

**Documentation Impact:**

- [x] CLAUDE.md — add `internal/visibility` to the package table + a "read-out visibility wrappers" rule pointing at DEC-ZBI39P
- [x] Package godoc carries the full contract (gate-first, stored-type check, hidden=nonexistent, read-out-only, capability-not-identity, fail-closed redactor, timing residual)
- [x] ~~docs/metamodel.md, cli-reference.md, data-entry.md~~ (N/A: no user-facing change in this PR)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-SRZK6X (critical — stored-type check moved
in-package), RR-6IL3X7 (significant — alias-safe map replacement), RR-5N4K35
(significant — Title fallback on trace/path nodes), RR-Y7P4MQ (significant —
FilterRelations with both-endpoints rule added to PR 1), RR-MYLUSZ (minor —
batched orphan gating), RR-FJUQSF (minor — FieldRedactor fail-closed contract),
RR-7V9XN7 (nit — timing residual documented). All folded into Approach/AC/Test
Plan above.
