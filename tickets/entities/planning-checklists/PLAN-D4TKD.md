---
id: PLAN-D4TKD
type: planning-checklist
title: 'Planning: Extend predicate to a typed superset, then converge the filter/predicate evaluators'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: (Phase 1) Extend `internal/predicate` with typed values so it becomes a
superset of `internal/filter`'s comparison capability: integer, date/datetime
(instant-granular), list membership, and string glob/regex/fuzzy — the last
three as host functions, not language core. (Phase 2) Add a `predicate`-backed
`--filter <expr>` CLI flag; freeze `--where` as a transpiled-to-predicate legacy
alias; migrate automation `when:`/`validate:` and metamodel validation
`When:`/`Then:` onto `predicate` with a minimal **entity-only** env.

OUT: New host functions or extra context (`current_user`, `has_role`,
`old`/`new`) on the automation/validation envs — deferred. SQL pushdown of
filters (confirmed non-issue; matching is in-Go/post-fetch). Re-deciding the
RES-6PK0S3 data-shaping-vs-policy boundary in the abstract.

**Acceptance Criteria:**
1. **Integer ordering is numeric, not lexicographic.** `predicate.Compile(env, "entity.count > 9")` with `count` declared IntType, bound 10 -> true; 8 -> false. Table-driven.
2. **Date comparison is instant-granular and format-aware.** `entity.due < '2026-02-01'` compiles when `due` is DateType; `2026-01-15` -> true, `2026-03-01` -> false. Datetime carries time-of-day. Mirrors `filter/match_test.go` date cases.
3. **List membership.** `contains(entity.tags, 'urgent')` true iff bound list contains the element. Cases: present / absent / empty-list / nil.
4. **String glob/regex/fuzzy as host funcs.** `match(entity.name, '*.md')`, `regex(entity.name, '^RES-')`, `fuzzy(entity.title, 'authrztn')` behave as `filter`'s glob/`=~`/`~`. Parity cases lifted from `filter/match_test.go`.
5. **`--filter` full expression works end to end.** `rela list --filter "entity.status == 'ready' and entity.priority ~= 'low'"` returns the same set as two `--where` clauses, plus an OR the old syntax can't express.
6. **`--where` transpile is behavior-identical.** For every existing `filter/match_test.go` case, the transpiled predicate yields the same verdict. Golden test: run the `filter` corpus through both engines, assert equal.
7. **Automation/validation migration preserves current typed behavior + adds `or`.** This repo's `metamodel.yaml` automations/validations evaluate identically (the typed `filter.Match` behavior already in `engine.go:357 matchProperty` must not regress); a new `or`-using rule now works. Golden replay of the real config.
8. **Invariant preserved: no I/O / no metamodel lookup at Eval.** `predicate/arch_test.go` forbidden-import list still passes; date parsing at compile/bind, not eval.

## Research

- [x] RES-6PK0S3 covers this exact question (linked `has-research`). It recommended "keep two" *because predicate was not a superset*; its Option C precondition (predicate proven on the read path — the affordance resolver, TKT-9E57) is now met.
- [x] No external library — task is unifying two in-tree engines.
- [x] Codebase prior art surveyed (file:line below).
- [x] Concepts reviewed: `metamodel-types` (affects), authorization (predicate's home).

**Existing Solutions / prior art (file:line):**
- `internal/predicate/value.go` — sealed `Value` sum type. `List`/`Record` **values already exist** (89, 62); `ListType`/`RecordType` **type descriptors already exist** (env.go 44, 63). Gap is grammar + a Date/Int variant, not scaffolding.
- `internal/predicate/eval.go:214 evalOrdered` — single site ordered comparison dispatches on value type; Date/Int slot in beside Number/String. `valuesEqual:190` similarly.
- `internal/predicate/env.go:129 DeclareFunc` — **RR-93UN**: host funcs return scalars only (runtime check doesn't re-validate nested Record/List fields). glob/regex/fuzzy return bool -> fine.
- `internal/filter/match.go` — the full behavior phase 2 must preserve: `matchDate:272` (instant-granular, accepts string|time.Time, parses via `metamodel.ParseDateValue`), `matchInteger:313`, `matchEnum:365` (validates value ∈ allowed), `matchList:101` (any-element `=`, no-element `!=`), string glob/regex/fuzzy `matchString:198`, and the **empty/missing-value contract** (30-63).
- `internal/automation/engine.go:357 matchProperty` — automation `when:`/`validate:` **already** routes typed comparisons through `filter.Match` (RES-6PK0S3 fix landed; measure `automation-typed-comparison-test` pins it). Migration must preserve this.
- `internal/metamodel/types.go:530 (AutomationTrigger.When)`, `62-71 (ValidationRule.When/Then)` — YAML surfaces to migrate.

## Approach

- [x] Chosen and documented
- [x] Builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Phase 1 — extend predicate (the real work):**
1. Add `Int` and `Date` variants to the sealed `Value` sum type + `IntType`/`DateType` descriptors. `Date` boxes a `time.Time`. (List already exists.)
2. Extend `evalOrdered` (eval.go:214) with Int (int64 compare) and Date (`time.Time` Before/After/Equal, mirroring match.go:294); extend `valuesEqual` (eval.go:190).
3. **Type context enters at the compile-time Env, not eval** (resolves the open fork — option a). Caller declares `entity` as `RecordType{"due": DateType, "count": IntType, ...}` built from metamodel `PropertyDef`s at Env-construction. A metamodel->Env adapter (new, small; wiring-site, mirrors acl `MetamodelView` — predicate must NOT import metamodel, arch_test forbids it). **Date/int RHS literals are written as string/number literals and coerced at compile against the field's declared type** (e.g. `entity.due < '2026-02-01'`: RHS string parsed to Date at compile because LHS is DateType — the `matchDate` trick, moved to compile so eval stays pure). Crux: type-checker literal coercion in `walkRelational`.
4. glob/regex/fuzzy as **host functions** (`match`, `regex`, `fuzzy`) returning bool — no core grammar change; reuse `filter`'s `TrigramSimilarity` / glob->regex. Add `contains(list, elem) bool` and `today()`/date-builder host func.

**Phase 2 — converge (only after Phase 1 lands cleanly):**
5. `--filter` flag: parse with predicate + entity-only Env; evaluate per candidate. `--where` stays; `filter.Parse` result -> transpile to predicate AST/source (subset mapping total — AC6). **Values map to typed literals via the field's declared type** (correction to earlier ticket note: always-string would *lose* the typed comparison automations already have; transpile must honor declared types to preserve `count>9` numeric).
6. Migrate `automation.Engine` (validation + `matchesWhenConditions`) and `internal/validation` to compile each `when:`/`then:`/`check` string through predicate once (cache compiled `Program`s), evaluate per event. Keep accepting legacy filter strings via transpile-on-load + one-time deprecation `slog.Warn`.

**Files to modify:** `internal/predicate/{value.go, env.go, eval.go, ir.go,
compile.go}`; new predicate host-func registration; new metamodel->predicate.Env
adapter (wiring site); `internal/cli/list.go` (+ `filter_helpers.go`);
`internal/automation/engine.go`; `internal/validation/validation.go`;
`internal/filter/` (+ `ToPredicate` transpiler).

## Security Considerations

- [x] Input sources identified
- [x] Validation approach defined (allowlist)
- [x] Security-sensitive operations identified
- [x] Errors don't leak sensitive info

**Input Sources & Validation:**
- Expressions come from **operator-controlled config** (metamodel.yaml, acl.yaml, operator CLI args) — NOT end-user request bodies. `--filter`/`--where` are typed by whoever runs `rela` (same trust as today's `--where`). predicate is already hardened: allow-list walker (compile.go:294 default-reject), compile depth budget (256), eval step budget (10 000), no I/O. New host funcs must not undo this.
- **Regex host func = sharpest new risk (ReDoS).** Mitigation: Go `regexp` is RE2 (non-backtracking) -> no catastrophic ReDoS by construction. Confirm fuzzy/glob compile through `regexp` too. The step budget does NOT bound time inside a single host call, so RE2's linear guarantee is **load-bearing** — document it. **(-> RR: confirm RE2 path, significant.)**
- **Date parse errors** surface as compile errors with the expected format (like match.go:290); keep messages format-hint-only, don't echo full attacker values into leaky contexts.

**Security-Sensitive Operations:** none new beyond expression eval; no
file/net/crypto. The no-I/O-at-eval invariant (AC8) is the security-relevant one
(keeps predicate ACL-read-path-safe) — must not break (date parse at
compile/bind, never eval).

## Test Plan

- [x] Scenarios per AC
- [x] Edge cases identified
- [x] Negative tests defined
- [x] Integration approach defined

**Highest-value: cross-engine parity (AC6/AC7).** Replay `internal/filter`'s
`match_test.go` corpus and this repo's `metamodel.yaml` automations through both
old and new engines; assert identical verdicts. This is the migration gate, not
unit tests alone.

**Edge Cases:**
- Empty/missing property: predicate `evalAttr` returns `NewNil()` for declared-but-absent field (eval.go:117). Must reconcile with `filter`'s contract (match.go:30-63): `prop=''` matches empty; `prop~=''` matches present; missing matches neither `==value` nor `~=value`. Transpiler must map `prop=`/`prop!=` to the exact nil/empty predicate. **Subtlest parity risk. (-> RR, significant.)**
- Date: nil, malformed literal (compile error), datetime vs date granularity, timezone (mirror `filter`; invent no new tz semantics).
- Integer: negative, zero, MAX_INT — int64 not float64 (the whole point); verify no float64 round-trip in the compile path.
- List: empty, nil, `contains` on non-list (compile-time type error), `!=`/no-element semantics.
- Enum: RHS not in allowed set — `filter` errors (match.go:392). **Decide predicate parity (error vs false) and pin it. (-> RR, significant: enum-validation gap.)**
- Concurrency: `Program` immutable + Eval-concurrent-safe (doc.go); new values immutable (List/Record retain-by-reference — no post-construct mutation, RR-AJS4).

**Negative Tests:** ordered op on bool/enum -> compile error; `contains` on
scalar -> compile error; malformed date literal -> compile error naming the
format; invalid regex pattern -> compile error, not eval panic.

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- **Literal coercion in the walker is the hard part** (date/int RHS typed by LHS field type at compile). Isolate in `walkRelational`; extensive parity tests. Fallback = bind-time typed values (rejected fork b) — re-scatters metamodel-aware typing, so prefer coercion.
- **Enum/status/priority semantics**: `filter` validates RHS ∈ allowed and errors on invalid; reproducing needs the Env to know enum members. Recommend plain-string in phase 1 (loses validation), note the gap.
- **Parity completeness**: `filter`'s empty/missing contract is fiddly; a transpile that misses it silently changes automation/validation outcomes. Golden replay of real config is the gate.
- **Effort L–XL.** Phase 1 alone L (type system + walker coercion + host funcs + parity). Phase 2 M once Phase 1 solid. **Ship Phase 1 as its own PR, Phase 2 behind it.**

**Effort:** l (phase 1), then m (phase 2); xl if combined — prefer split.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md — `when:`/`validate:`/`When:`/`Then:` accept full predicate expressions (+ legacy compat + deprecation note).
- [x] docs/cli-reference.md — new `--filter` flag; `--where` deprecation note.
- [x] CLAUDE.md — update the "two condition languages" model to "predicate is the condition/policy engine; filter is the frozen legacy query-string subset."
- [x] internal/predicate/doc.go — new types, host funcs, compile-time-typing / no-I/O-at-eval invariant.

## Design Review

- [x] Run `/design-review` — done; findings below folded into this plan.
- [ ] All critical/significant findings addressed in plan (review-responses to be created)

**Design Review Findings (this pass):**
1. *(significant)* Empty/missing-value parity — `filter`'s (match.go:30-63) contract vs predicate's `NewNil()` on absent field must be reconciled by the transpiler exactly, or automation/validation verdicts silently drift.
2. *(significant)* Regex host func ReDoS — safe only because Go `regexp` is RE2 (non-backtracking); step budget doesn't bound a single host call. Confirm + document the RE2 guarantee; ensure glob/fuzzy also compile through `regexp`.
3. *(significant)* Enum RHS validation gap — `filter` errors on invalid enum value; plain-string predicate would silently not-match. Decide + pin parity.
4. *(significant)* Literal coercion approach — date/int RHS literal typing by LHS field type at compile is the load-bearing design choice; if it proves infeasible the fallback (bind-time typed values) re-scatters metamodel logic. Prototype `walkRelational` coercion first.
5. *(minor)* Integer must be true int64 end-to-end — no float64 round-trip in the compile path (predicate's Number is float64; Int must be a distinct variant, not a re-typed Number).
6. *(minor)* Phase split — Phase 1 and Phase 2 should be separate PRs; the plan/AC already separate them, keep them separate in delivery.
