---
id: PLAN-E7S385
type: planning-checklist
title: 'Planning: Converge condition evaluators on predicate: --filter CLI, unify type adapter, migrate automation/validation (TKT-7EJK4 phase 2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (in ticket TKT-MOCIED)
- [x] Acceptance criteria documented with test scenarios

**Scope:** see TKT-MOCIED. Phase 2 converges condition evaluators on
`internal/predicate`. Four work items: (1) unify the metamodel→predicate.Type
adapter — migrate live `affordances` onto `predicatefns.EntityRecordType`; (2)
`filter`→predicate transpiler; (3) `--filter` CLI flag; (4) migrate
automation/validation onto predicate. Deferred Phase-1 RRs folded: RR-9V6PF
(pattern cache), RR-G3Y70 (negative literals), RR-XJBGB (enum validation —
revisit).

**Acceptance Criteria:**
1. **Adapter unified.** `affordances` builds its entity `RecordType` via `predicatefns.EntityRecordType` and binds `NewInt`/`NewDate`; its own `scalarPredicateType`/`coerceNumber`-for-integer path is gone. Existing affordance `when:` behavior unchanged (integer/date `when:` predicates still evaluate correctly). Test: affordances suite green + a new case with an integer/date `when:`.
2. **Transpiler total + parity.** `filter.ToPredicate` maps every `filter/match_test.go` corpus case to a predicate that yields the identical verdict (golden replay both engines). Empty/missing contract matches `parity_missing_test.go`. Typed literals honor declared type (`count>9` numeric, `due<'...'` date).
3. **`--filter` works.** `rela list --filter "entity.status == 'ready' and entity.priority ~= 'low'"` returns the expected set incl. an `or` the old syntax can't express. `--where` still works (transpiled), with a one-time deprecation notice.
4. **Automation/validation migrated + typed behavior preserved.** This repo's `metamodel.yaml` automations/validations evaluate identically post-migration (measure `automation-typed-comparison-test` stays green); a new `or`-using rule works. Golden replay of the real config.
5. **No new eval-time I/O.** predicate `arch_test` still green; date parse at compile/bind only.

## Research

- [x] RES-6PK0S3 (has-research) — the converge decision this completes.
- [x] Re-surveyed current landscape (post-Phase-1-merge) — file:line map in ticket body. Key correction vs original plan: the crux is the **affordances adapter conflict**, not automation; `conditionlint`/`statemachine` already use predicate (no third language); `predicatefns` is production-unused today.

## Approach

- [x] Chosen; builds on Phase 1 primitives; alternatives considered

**Sequencing (risk-first; each an independently shippable slice, own PR if
large):**

1. **Adapter unification (crux, do first).** In `internal/affordances`: replace the local `scalarPredicateType` (env.go:104-127) with a call to `predicatefns.EntityRecordType`, and change the binder `coerceScalar`/`coerceNumber` (bindings.go:167-199) so integer→`NewInt`, date/datetime→`NewDate` (parsed against the field layout, matching how `predicatefns` types them). **Adapter and binder MUST change together** (RR-4189H: a declared `IntType` field bound with a `Number` fails `runtimeTypeAccepts` at eval → affordance error). Arch: affordances already `mayDependOn predicatefns`? — verify; if not, add. Keep affordances' list/record handling; only the scalar int/date mapping moves. Statemachine's Env (predicate.go:19-52) also declares an `entity` record — route it through the shared adapter too, or leave if it only declares string fields (verify).

2. **`filter`→predicate transpiler** (`internal/filter/topredicate.go`, new). `filter.Filter`/`[]*Filter` → predicate source string (or AST). Total subset mapping: `k=v`→`entity.k == 'v'` (typed by declared type), `k!=`→presence-guarded, empty/missing per `parity_missing_test.go` mapping table, glob/regex/fuzzy→`match`/`regex`/`fuzzy(...)` host-fn calls, list→`contains(...)`. Needs the entity's `PropertyDef`s for typing (same input `EntityRecordType` takes). Arch: `filter` mayDependOn `predicate`? verify/add (predicate stays leaf; filter→predicate is fine, no cycle since predicate `mayDependOn: []`).

3. **`--filter` CLI flag** (`internal/cli/list.go`). New `Filter []string` kong flag alongside `Where`. Compile once via `predicate.Compile` + `predicatefns.Declare`, evaluate per candidate entity with `predicatefns.Bind`. `--where` path routes through the transpiler (deprecation `slog.Warn` once). Fold **RR-9V6PF**: constant-literal pattern validation at compile + a Program-level cache so `--filter` doesn't recompile per entity. **RR-G3Y70** (negative literals) surfaces here — decide: allow negative numeric literals in the walker (small grammar change) vs document the gap. Recommend the grammar fix since `--filter` makes it user-visible.

4. **Migrate automation + validation** (`automation/engine.go`, `validation/validation.go`). Compile each `when:`/`then:`/`check` once (cache Program keyed by source+type), evaluate per event/entity with an **entity-only** Env. Legacy filter-strings accepted via transpiler-on-load + one-time deprecation warning. Preserve typed behavior. Entity-only Env — NO `current_user`/`old`/`new` (deferred).

**Files:** `internal/affordances/{env,bindings}.go`;
`internal/filter/topredicate.go` (new) + tests; `internal/cli/list.go` (+
predicate cache helper); `internal/automation/engine.go`;
`internal/validation/validation.go`; arch-lint deps for filter→predicate and
affordances→predicatefns; docs.

**Alternatives rejected:** (a) keep two adapters and only add `--filter` —
leaves RR-TBG91 divergence live and predicatefns unused, defeats "one engine".
(b) transpile at eval time instead of parse/load — re-parses per row;
compile-once is the point.

## Security Considerations

- [x] Input sources / validation / sensitive ops / error leakage

**Input Sources:** predicate/filter expressions are **operator-controlled
config** (metamodel.yaml, acl.yaml) and operator CLI args
(`--filter`/`--where`), same trust as today's `--where`. No end-user request
bodies. predicate hardening (allow-list walker, depth/step budgets, no eval I/O)
carries over. New host-fn patterns route through RE2 (RR-N176T, already enforced
in predicatefns). The affordance path is the ACL read gate — the migration must
NOT weaken it: the unified adapter/binder must produce the SAME verdicts
(fail-closed on eval error is existing affordances behavior; keep it).
**Sensitive op:** affordances is on the ACL read path — an adapter bug that
changes a `when:` verdict changes who can see/edit a field. AC1 + golden
affordance tests are the gate.

## Test Plan

- [x] Scenarios per AC; edge cases; negative; integration

**Highest-value: cross-engine golden parity.** (a) Replay `filter/match_test.go`
corpus through `filter.Match` vs `ToPredicate`→predicate, assert identical
(AC2). (b) Replay this repo's `metamodel.yaml` automations/validations through
old (filter) vs new (predicate) path, assert identical verdicts (AC4). (c)
Affordances: existing suite + integer/date `when:` cases proving the adapter
swap preserves verdicts (AC1). **Edge cases:** empty/missing
(parity_missing_test target), int overflow (Phase-1 guard), negative literals
(RR-G3Y70 decision), date layout/tz (Phase-1), enum RHS (RR-XJBGB —
plain-string; pin behavior), list `contains`, glob/regex/fuzzy parity,
`or`/`and` composition. **Negative:** malformed `--filter` expr → clear compile
error, not panic; transpiler on an un-typable property → error surfaced.

## Risk Assessment

- [x] Risks + mitigations + effort

**Risks:**
- **Adapter swap changes an ACL affordance verdict** (silent authz change). Mitigation: golden affordance verdict tests before/after; adapter+binder change atomically; fail-closed preserved. HIGH — gates the slice.
- **Transpiler parity gaps** (empty/missing, typed literals) silently change automation/validation outcomes. Mitigation: golden replay of real config is the gate.
- **Transpiler parity gaps** (empty/missing, typed literals) silently change automation/validation outcomes. Mitigation: golden replay of real config is the gate.
- **Program cache correctness.** Cache key must be scoped to the metamodel/resolver instance, NOT process-global `(source,type)` — `EntityRecordType` bakes the field date layout + field set into the type, so two projects share a key but need different Programs (**RR-2Y851X**). Mirror affordances' per-resolver `envs map[string]*predicate.Env` (resolver.go:69).
- **Effort: L–XL.** Slice 1 (adapter) M and risky; slice 2 (transpiler) M; slice 3 (CLI) S–M; slice 4 (migration) M. **Ship as ≥2 PRs**: PR-A = adapter unification (closes RR-TBG91, independently valuable); PR-B = transpiler + `--filter` + migration. Possibly split PR-B further.

**Effort:** xl overall; deliver in slices.

## Documentation Planning

- [x] User-facing docs identified

- [x] docs/cli-reference.md — `--filter` flag; `--where` deprecation note.
- [x] docs/metamodel.md — `when:`/`validate:`/`When:`/`Then:` accept predicate expressions (+ legacy compat/deprecation).
- [x] CLAUDE.md — "predicate is THE condition engine; filter is the frozen legacy query-string subset, transpiled." Update the two-adapter note (now one).
- [x] predicate/doc.go, predicatefns docs — unified adapter as the one entry point.

## Design Review

- [x] Run `/design-review` before implementation — done (cranky, adversarial pass over the plan against live code)
- [x] All critical/significant findings folded into the plan below

**Design-review findings (3 critical, 4 significant, 4 minor) — plan
corrections:**

- **RR-WHMVLW (critical) — date binder MUST handle `time.Time`.** YAML decodes unquoted dates to `time.Time`, quoted to `string`. The new date binder must bind `time.Time` directly to `NewDate`, parse `string` via `metamodel.ParseDateValue(propDef)` (NOT a hand-rolled layout), else `Nil`. Missing the `time.Time` case silently flips ACL grants to deny. Golden test with BOTH quoted+unquoted date frontmatter. **Corrects slice-1 approach.**
- **RR-782ULH (critical) — the date swap is a SEMANTIC change, not neutral.** date StringType→DateType turns lexicographic string compares into instant-granular `time.Time` ordering — a real verdict change on the ACL boundary. **AC1 reframed:** intentional lexicographic→instant shift; test with adversarial stored values (`'2026-1-5'`, `'2026-01-05T23:59:59Z'`) that DISTINGUISH the two orderings, not happy-path ISO dates.
- **RR-TQEHO4 (critical) — transpiler home + AST route.** It CANNOT live in `internal/filter` (needs predicatefns func-names; predicatefns imports filter → cycle). **Transpiler goes in `internal/predicatefns` as `FromFilter(...)`.** Build the predicate **AST/IR directly**, NOT a Lua source string (a filter value with `'`/`\`/newline breaks out of the Lua string literal — injection/mis-parse). **Corrects slice-2 file list + arch delta.**
- **RR-IRV2WJ (significant) — int binder keeps string→int coercion.** `resolver_test.go:358` pins that a string-stored `"5"` on an int field coerces. New int binder must accept int/int64/float64-integral/**string** (like `coerceNumber`). Keep that test green; add a `time.Time`-for-date sibling.
- **RR-NKWJS6 (significant) — mapping is NOT total.** fuzzy-with-wildcard (two-phase), glob-`!=`, list-`!=` (NONE vs contains=ANY) have no clean/faithful host-fn equivalent. **AC2 reframed:** enumerate the operator×type matrix; unsupported cases (fuzzy-with-wildcard) → transpiler returns a clear ERROR, never a silently-different predicate. Gate: every `match_test.go` case maps OR is explicitly rejected.
- **RR-2Y851X (significant) — cache scoping** (folded into Risk above).
- **RR-02P03I (significant) — name the unmigrated filter consumers.** Added to Scope-OUT below.
- **M1 (minor) — statemachine: LEAVE AS-IS.** Its Env declares only `{id,type,value}` all StringType (predicate.go:26-30); routing through `EntityRecordType` would expand its surface (behavior change). Resolves the slice-1 hedge → do NOT touch statemachine.
- **M4 (minor) — RR-G3Y70 negative literals** is "allow negative numeric *literals*" only (not general unary minus); round-trips through `coerceIntLiteral`'s existing `±maxExactIntLiteral` guard.

**Scope-OUT addition (RR-02P03I) — filter consumers NOT migrated in Phase 2
(stay on `filter.Match`, unchanged):**
`internal/dataentry/{views,feed_provider,helpers}.go` (SPA view/feed `where:`),
`internal/lua/runtime.go` (script queries), `internal/search/searchparser`
(search property clauses), `internal/cli/analyze.go`. CLAUDE.md doc must say
"predicate is the condition engine; filter still backs query-filtering in these
subsystems" — NOT "filter is fully frozen."
