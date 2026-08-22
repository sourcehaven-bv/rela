---
id: PLAN-FSMON5
type: planning-checklist
title: 'Planning: Extract internal/expr from internal/predicate, then give automations date arithmetic'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Revision 2** (2026-08-16) — rewritten after `/design-review`. Six findings
> (C1–C4, S1, S2) were fixed in the plan; four were factual errors about the
> code. See "Design Review" at the bottom for the mapping.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Two separable problems, sequenced in one ticket because the second depends on
the first.

**(a) `internal/predicate` is misnamed.** It is a general typed expression
engine (own compiler, typed IR, `Value` over
Bool/Number/String/Date/Record/List, `DeclareFunc`, linter, fuzz tests) with a
one-line Bool restriction at the top level
(`internal/predicate/compile.go:109`). `Program.Eval` already returns `(Value,
error)`. Nothing below that line is Bool-specific.

**(b) Automation conditions cannot do date arithmetic.** `when:` is evaluated by
`filter.Match` (`internal/automation/engine.go:381`), a `key op value` DSL with
no functions and no composition. "Due within N days" is inexpressible.

**Scope:**

IN:

1. Move the general engine to `internal/expr` (11 non-test files, ~1,950 LOC;
plus 8 test files and `testdata/` — see Files).
2. Keep `internal/predicate` as a **type-alias facade**. `Program` is an alias,
so `Eval` **keeps returning `(Value, error)`** (C1/C2). Bool ergonomics come
from a new additive `predicate.EvalBool`. `lint.go` moves to `expr`; `doc.go`
splits.
3. Add `ExpectType` as a `CompileOption`, **restricted to scalar types** with
its argument validated at `Compile` entry (C4). Default stays `BoolType`.
4. Add `date_add`, `days_between`, `rrule_next` to `internal/predicatefns`,
**plus a new `BindEntity` binder** congruent with `EntityRecordType` (S2).
5. Add an opt-in `condition:` key to `AutomationTrigger`, evaluated by the
expression engine, ANDed with the existing `when:`. Env declares **`new` and
`old`** (S1), not `entity`.
6. `NewEngineFromMetamodel` gains an `error` return so a bad `condition:` is a
**load-time error** (C3). Thread through its two callers.
7. **Validation rules** (`validation.go:163,167` + `AutomationCheck.Check` at
`engine.go:329`) get the same `condition:` support and the same fail-loud
posture — free once (6) lands, and avoids changing the constructor twice.
*(Scope decision reversed by user 2026-08-16; was OUT in revision 1.)*

OUT (deliberately deferred):

- `{{...}}` as an expression context for `set:`/`value:`. Today it is pure
variable substitution; making it evaluate expressions touches every existing
automation and needs its own decision (`{{ }}` vs `{{= }}`). Separate ticket.
- Scheduler / time-triggered automations. The follow-up this unblocks.
- Renaming `internal/predicatefns`.
- Binary `+`/`-` operators on dates.
- Migrating existing `when:`/`then:` filter strings to expression syntax. The
dual-key design is deliberate — see Alternatives.

**Acceptance Criteria:**

1. `internal/expr` holds the general engine (incl. `lint.go`);
`internal/predicate` holds the alias facade + `EvalBool` + a pointer `doc.go`. —
*Test: `just arch-lint` (an arch rule, not a file-layout assertion — see M3).*
2. The five importers (`affordances`, `conditionlint`, `metamodel`,
`predicatefns`, `statemachine`) compile with **zero call-site edits**. — *Test:
`go build ./...` green with no diff outside `predicate/` and `expr/` after the
facade commit. This is the proof, per L2.*
3. `expr.Compile(env, src)` with no option still rejects a non-Bool top-level
expression; `ExpectType(DateType)` accepts a Date-returning expression and
rejects a Bool one — both at **compile**. `ExpectType(DateType)` also accepts a
`DateTypeWithLayout` result. `ExpectType` rejects `RecordType`, `ListType` and
`AnyType` as arguments with a clear error. — *Test: table-driven.*
4. `predicatefns` gains `date_add`, `days_between`, `rrule_next` with typed
signatures, and `BindEntity`. A malformed RRULE fails at **Eval** with a clear
error (NOT at compile — see S5). — *Test: signature assertions; malformed-RRULE
eval-error case; the round-trip test in AC5.*
5. **Every metamodel property type** round-trips through `EntityRecordType` +
`BindEntity` and passes the runtime type check. — *Test: table over all property
types. This is the highest-value test in the ticket (S2).*
6. An automation `condition:` evaluates `days_between(new.due, today()) <= 7`
correctly, with `old` bound to all-`Nil` on create. — *Test: due / not-due /
create-event fixtures through the engine.*
7. A `condition:` that fails to compile is a **load-time error** surfaced
through `NewEngineFromMetamodel` → `buildAutomation`; it is NOT silently
skipped. — *Test: bad-expression metamodel → `appbuild` returns an error.*
8. `just arch-lint`, `just test`, `just lint`, `just coverage-check` pass.
9. No behaviour change for any existing predicate caller — pinned by the
existing `affordances` / `conditionlint` / `statemachine` suites, unmodified.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-6PK0S3 (pre-existing, linked via `has-research`) — "Should
filters, views, automations, and search converge on one comparison evaluator?"

**Existing Solutions:**

*No external library considered.* `internal/predicate` is already a hand-written
typed expression engine with its own compiler, IR, linter and fuzz tests, built
deliberately (TKT-7EJK4). Introducing `expr-lang` or similar would discard a
sandboxed, budgeted, ACL-audited engine for one with a larger attack surface.
The work here is extraction, not replacement.

*Decisive prior-art finding — the dialect is NOT automation-specific.*
`internal/filter` is parsed at 18 non-test sites across 8 user-facing surfaces:

| Surface | Site | Authored by |
|---|---|---|
| CLI `--where` | `cli/list.go:91` | users, at the shell |
| View filters | `dataentry/views.go:195` | `data-entry.yaml` |
| Calendar feeds | `feed_provider.go:80,126,177` | `data-entry.yaml` |
| CalDAV collections | `caldav_backend.go:204` | `data-entry.yaml` |
| Lua `list_entities` | `lua/runtime.go:1086` | every script |
| Search `prop:` terms | `searchparser/parser.go:67,84` | users, in the box |
| Validation `when:`/`then:` | `validation/validation.go:163,167` | `metamodel.yaml` |
| Automation `when:` | `automation/engine.go:61,329` | `metamodel.yaml` |

This killed the two approaches favoured before checking — see Alternatives.

*Reusable code found:*

- `internal/predicatefns` **already ships `today()`** (DateType, UTC-truncated,
`now` injected for purity — RR-YPYTP), plus `match`, `regex`, `fuzzy`,
`contains`. **`Declare`/`Bind` have zero production callers** — the stdlib is
built, tested, documented and unwired. Phase 3 extends it.
- `metamodel.ValidateRrule` (`internal/metamodel/rrule.go:17`) exists and is
already used by the Lua `rrule_next` binding.
- `CompileOption` already exists (`compile.go:13`, alongside `WithMaxDepth`), so
`ExpectType` is additive.
- `Engine` already holds `*metamodel.Metamodel` via `SetMetamodel`
(`engine.go:50`), so phase 4 needs no new plumbing to reach the schema.
- `CompileAll` (`lint.go:26`) already does exactly the batch fail-fast that
phase 4 needs — which is why it moves to `expr` (S3).

*Anti-reuse finding (S2).* `predicatefns.EntityRecordType` is **NOT** paired
with a usable binder. Its own godoc (`predicatefns/env.go:29-38`) warns it is
incompatible with the affordances binder (RR-TBG91): affordances predates the
Int/Date value types and maps integer→Number, date→String. There are **three**
divergent metamodel→type adapters (`predicatefns/env.go:56`,
`affordances/env.go:108`, `conditionlint.go:113`). Phase 4 must use the
`predicatefns` one (only it has `DateTypeWithLayout`, which `days_between`
needs) and must ship a new congruent binder.

*Concepts reviewed:* `metamodel-types` (affected), FEAT-013 (implemented).
Decisions carried forward: RR-A3EZR, RR-N176T, RR-YPYTP, RR-TBG91.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Chosen: additive sibling key.* `when:` stays byte-identical; automations gain
an opt-in expression key that ANDs with it.

```yaml
on:
  entity: terugkerend
  when: ["status=actief"]                                 # filter, unchanged
  condition: "days_between(new.volgende_datum, today()) <= 0"   # expression
```

Two dialects, but visibly distinct keys with distinct jobs — matching the
architecture that already exists rather than fighting it. `RES-6PK0S3`'s
boundary (**filter = data-shaping, predicate = policy**) stands.

*Phase 1 — extract `internal/expr`.* Two commits, mechanically verifiable (L1):

- **Commit A (pure move).** `git mv` the 11 non-test files + 8 test files +
`testdata/` to `internal/expr`; rewrite the package clause and import paths.
Nothing else. If commit A's diff contains anything but package/import changes,
the "pure move" claim is false.
- **Commit B (facade).** Add `internal/predicate` as ~30 lines of type aliases
plus `Compile`, `CompileAll`, `WithMaxDepth`, `WithStepBudget`, and the new
`EvalBool`.

**`Program` is a type ALIAS** (`type Program = expr.Program`), so `Eval` keeps
`(Value, error)` (C1/C2). A wrapper struct would break `*predicate.Program`
struct fields at `affordances/affordances.go:46,53,65` and
`statemachine/statemachine.go:109`, force `Compile`/`CompileAll` to allocate,
and require preserving `CompileAll`'s `progs[i] == nil` contract (pinned by
`lint_test.go:41-52`) through the wrapping. Aliases avoid all of it.

Bool ergonomics arrive as a **new free function**, not a signature change:

```go
func EvalBool(ctx context.Context, p *Program, b *Bindings) (bool, error)
```

Additive, breaks nothing. The two existing call sites
(`affordances/resolver.go:754`, `statemachine/predicate.go:74`) keep their
`v.(predicate.Bool)` type assertions **and their fail-closed non-bool branches**
— in affordances that branch is a security decision ("did not return bool → deny
grant"), not dead code. They may migrate to `EvalBool` later, separately.

`lint.go` **moves to `expr`** (S3): `CompileAll` is engine machinery, its
purpose is load-time batch fail-fast, and phase 4 needs it with `ExpectType`.
Left in `predicate` it would be hardcoded to Bool and phase 4 would have to
reimplement the loop. `Issue`/`NamedSource` are generic too.

`doc.go` splits: the bulk (numeric model, coercion, security model, budgets)
describes the general engine and goes to `expr/doc.go`; `predicate/doc.go`
becomes a short pointer. There is no `doc_test.go` and no testable examples —
verified.

`arch_test.go` is **duplicated, not moved**: `expr` needs the identical
forbidden-import list, and `predicate` still needs one so the facade cannot
quietly grow a metamodel import.

**Error prefix:** keep the literal `"predicate: "` prefix in `expr`'s error
strings for this ticket. The 15 `testdata/reject/*.want` files match on reason
substrings only (verified — none contain `"predicate:"`), but `conditionlint`
surfaces these strings to operators (`conditionlint.go:81-90`), so changing them
would make a "pure move" user-visible. Rename later if desired.

*Phase 2 — `ExpectType`.* Replace the hardcoded assertion at `compile.go:109`
with the configured expected type; default `BoolType`. **Validate the option's
argument at `Compile` entry** (C4), rejecting `RecordType`, `ListType` and
`AnyType` — mirroring `DeclareFunc`'s existing rejection of Record/List returns
(`env.go:168-173`). Rationale: `equalsType` is asymmetric — `AnyType.equalsType`
matches only literal `AnyType`, so `ExpectType(AnyType)` would reject everything
(the opposite of the obvious reading), and `RecordType` demands exact field-set
equality. `DateType` deliberately ignores layout (`env.go:67-70`), so
`ExpectType(DateType)` accepts a `DateTypeWithLayout` result — load-bearing for
`rrule_next`, and pinned by a test.

*Phase 3 — date/RRULE host functions + binder* in `predicatefns`:

- `date_add(d Date, n Number, unit String) -> Date`
- `days_between(a Date, b Date) -> Number`
- `rrule_next(rule String, after Date) -> Date`
- `BindEntity(b *Bindings, def *metamodel.EntityDef, e *entity.Entity) error` —
**new**, emitting `NewInt`/`NewDate`/`NewString`/`NewBool` congruent with
`EntityRecordType` (S2).

`unit` is restricted to `day`/`week` for v1 (M4) — it covers the motivating "due
within N days" case and dodges `AddDate`'s surprising month normalization (Jan
31 + 1 month → Mar 2/3). Months can be added later with explicit
clamp-to-end-of-month semantics.

RRULE validation happens at **Eval**, not compile (S5). `FuncSig` type-checks
types, not values, and there is no per-argument compile-time hook; moreover
`rrule_next(new.schedule, ...)` takes its rule from a property, which could
never be compile-validated. The eval-time error must be clear and name the
offending rule.

*Phase 4 — wire into automation + validation.* Per automation, per entity type:
build env via `EntityRecordType` → `predicatefns.Declare` → compile the
`condition:` **once at load** → `BindEntity` + `Bind(now)` + eval per event.
`*Program` is immutable and safe for concurrent `Eval` (pinned by
`concurrent_test.go`); compiling per event would re-parse on every write.

**Env variables are `new` and `old`** (S1), matching the subsystem's existing
vocabulary — `{{new.title}}` interpolation, `from:`/`becomes:` trigger keys,
`Event.Entity`/`Event.OldEntity`. `entity` is deliberately NOT declared: a third
spelling for the same object, adjacent to old/new-aware keys, is an API that
cannot be changed after it ships. On create events `old` is declared as the same
`RecordType` but bound to all-`Nil`; `evalAttr` already returns `NewNil()` for a
missing field (`eval.go:113-121`), so `old.status == nil` works naturally.

**Eval errors mean "no match"**, matching `matchTyped`'s existing posture
(`engine.go:382-387`), plus a logged warning. A compile error is a load failure;
an eval error is a non-match. Both are pinned by tests.

**Fail loud at load.** `NewEngineFromMetamodel` gains an `error` return
(`engine.go:37`). Verified: it has exactly **two** production callers —
`appbuild.go:508` (inside `buildAutomation`, which already returns `error`) and
`appbuildtest/fixture.go:321` — so threading is contained. The existing silent
skip at `engine.go:61` is structural, not laziness: there was nowhere to put an
error. `condition:` must not inherit it.

For the `NewEngine(automations)` path (`engine.go:27`, tests/memory, no
metamodel): `condition:` cannot be compiled without a schema. It is a
**construction error** if any automation declares one — not a silent skip, which
would reintroduce exactly the bug being fixed.

**Files to modify:**

| File | Change |
|---|---|
| `internal/expr/*.go` | new — 11 non-test + 8 test files + `testdata/` moved |
| `internal/expr/lint.go` | moved from predicate (S3) |
| `internal/expr/doc.go` | bulk of predicate's doc.go |
| `internal/expr/arch_test.go` | duplicated from predicate |
| `internal/predicate/*.go` | shrink to alias facade + `EvalBool` + pointer doc |
| `internal/expr/compile.go` | `ExpectType` option + argument validation |
| `internal/predicatefns/predicatefns.go` | + 3 date/RRULE funcs |
| `internal/predicatefns/env.go` | + `BindEntity` |
| `internal/metamodel/types.go` | + `Condition string` on `AutomationTrigger`; same for the validation-rule type |
| `internal/automation/engine.go` | `NewEngineFromMetamodel` → `(*Engine, error)`; compile at load; eval per event; `AutomationCheck.Check` fail-loud |
| `internal/automation/types.go` | carry compiled `*expr.Program` |
| `internal/validation/validation.go` | `condition:` support + fail-loud (scope item 7) |
| `internal/appbuild/appbuild.go:508` | handle the new error |
| `internal/appbuild/appbuildtest/fixture.go:321` | handle the new error |
| `.go-arch-lint.yml` | `expr: mayDependOn: []` + `canUse: [gopherlua]`; `predicate: mayDependOn: [expr]`; `automation`/`validation` `+= predicatefns` |
| `docs/metamodel.md` | document `condition:`, the function set, `new`/`old`, and when to use `when:` vs `condition:` |
| `CLAUDE.md` | note the new package boundary |

**Alternatives considered:**

- **Migrate `when:` to expression syntax — REJECTED.** It either forks the
dialect (automations diverge from the other seven surfaces, worse than the dual
dialect it aimed to avoid) or cascades into all eight, breaking `--where` and
every Lua `list_entities` call. `RES-6PK0S3` costed this as Option C and rated
it **XL**.
- **Compat shim desugaring filter → expr — REJECTED.** Same reason: the
expression engine would have to reproduce filter's entire semantic surface
(glob, regex, fuzzy/trigram, enum, list-membership, date parsing) for all eight
callers, undoing predicate's deliberate minimalism.
- **`Program` as a wrapper struct with `Eval() bool` — REJECTED (C1/C2).**
Breaks struct fields in two packages, forces allocation in `Compile`, and
deletes two fail-closed security branches. Alias + `EvalBool` gets the
ergonomics additively.
- **Moving `affordances`/`statemachine` to import `expr` directly — DEFERRED.**
They genuinely use the general engine (28 distinct symbols; `predicate.Value`
alone at 26 uses), so this is arguably more honest. But it is a separate, purely
mechanical change and bundling it would break the "pure move" property of phase
- **Binary `+`/`-` operators on dates — REJECTED.** `Date + Number` is ambiguous
(days? months? calendar arithmetic is a policy decision), and operators tempt
eval-time coercion, breaking RR-A3EZR. Functions state the unit explicitly.
- **Key name `when_expr:` — REJECTED** in favour of `condition:`. Noted (M1)
that `conditionlint` already owns "condition" for this dialect, which is a point
*for* the name; the four-way vocabulary gets documented.
- **Compile-time RRULE validation — REJECTED as infeasible (S5)**; would need a
new `Validate func([]Value) error` on `FuncSig` invoked only for const args. Out
of scope.
- **Renaming `predicatefns` — DEFERRED.**

**Dependencies:** no new Go modules. Internal: `internal/expr` (new),
`predicate`, `predicatefns`, `metamodel`, `automation`, `validation`,
`appbuild`. `rrule-go` is already vendored (`metamodel/rrule.go`,
`lua/date.go`). Arch-lint verified no cycle: `predicatefns → {predicate, filter,
metamodel}` and `automation → metamodel` already coexist.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `condition:` expression | `metamodel.yaml` — **operator-authored, in-repo** | Compiled against a typed env at load; unknown variables/functions rejected by the existing allow-list walker | **Load-time error** (C3) |
| `ExpectType` argument | code, not config | Rejected if Record/List/Any (C4) | Compile error |
| RRULE string | config **or entity property** | `metamodel.ValidateRrule` at **Eval** (S5) | Eval error → no match + warning |
| Entity property values | the graph | Typed by the metamodel; bound via `BindEntity` congruent with `EntityRecordType` (S2) | Type mismatch is a bug, pinned by AC5 |
| `now` for `today()` | caller-injected | N/A — the engine never reads the wall clock at Eval | N/A |

The expression source is **configuration, not user input**: per root CLAUDE.md,
`metamodel.yaml` is operator-authored and already-disclosed. The trust boundary
is the same as every other automation action, including the existing `lua:` and
`lua_file:` actions — which are strictly more powerful. This ticket does not
widen it.

**Security-Sensitive Operations:**

- **Sandboxing is inherited, not rebuilt.** No I/O, no filesystem, no network;
an allow-listed AST subset bounded by step/depth budgets. Extraction must
preserve every one — the arch-lint rule (`expr: mayDependOn: []`) plus the
duplicated `arch_test.go` are how that is enforced.
- **The fail-closed branches at `affordances/resolver.go:758` and
`statemachine/predicate.go:78` must survive.** Keeping `Eval` returning `Value`
(C1) preserves them untouched. A `bool`-returning `Eval` would delete a
deny-by-default security decision as a side effect of a refactor.
- **RR-N176T (ReDoS)** — the step budget bounds IR steps, not wall-clock inside
a host call. `regex`/`glob`/`fuzzy` are safe only because Go's `regexp` is RE2.
The new date functions add no pattern matching, but the constraint must survive
the move.
- **RR-A3EZR** — date parsing at compile, never at Eval. The new host functions
compute over already-parsed `Date` values. `rrule_next` is the one exception (it
parses a rule string at Eval); it operates on a validated rule and returns a
single occurrence, not an unbounded expansion.
- **No new secrets, auth, crypto, or file access** on this path.

**Error handling:** compile errors name the offending expression and position —
operator-authored and already in the repo, so no disclosure concern. No entity
content is echoed into a compile error. Eval errors are logged as warnings and
must not include property values.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (AC → test)

| AC | Test |
|---|---|
| 1 | `just arch-lint` — the arch rule, not a file-layout assertion (M3) |
| 2 | `go build ./...` green with zero diff outside `predicate/`+`expr/` (L2) |
| 3 | Table over (source, option, want-error): no option + non-Bool → error; `ExpectType(DateType)` + Date → ok; + `DateTypeWithLayout` → ok; + Bool → error; `ExpectType(Record/List/Any)` → option error |
| 4 | Signature assertions; malformed RRULE → **eval** error |
| 5 | **Every** metamodel property type round-trips `EntityRecordType` + `BindEntity`, runtime type check passes |
| 6 | Engine-level: due / not-due / create-event fixtures for `days_between(new.due, today()) <= 7`; `old` all-`Nil` on create |
| 7 | Bad-expression metamodel → `appbuild` returns an error (not a skip) |
| 8 | `just arch-lint`, `just test`, `just lint`, `just coverage-check` |
| 9 | `affordances`/`conditionlint`/`statemachine` suites unmodified and green |

**Edge Cases:**

- **Timezone (RR-YPYTP).** `today()` is UTC-truncated to match date-literal
parsing. `days_between`/`date_add` must use the same convention. Explicit
local-midnight-vs-UTC boundary test.
- **`old` on create events** — declared, bound all-`Nil`; `old.status == nil`
is true. Pinned.
- **Missing/empty date property** — binds `Nil` (`eval.go:119`), so
`days_between(nil, today())` is an **eval** error → no match + warning.
- **Non-date property passed to a date function** — compile-time type error.
- **Both `when:` and `condition:` present** — AND; each side tested failing
independently. Absent `condition:` → `true`; present with `event.Entity == nil`
→ `false`, mirroring `matchesWhenConditions` (`engine.go:233,236`) (M2).
- **Evaluation order** — `condition:` is evaluated **after** the entity-type
gate and the event-type switch (`engine.go:162,174`), so a cheap mismatch never
pays expression cost (M2).
- **Undeclared property in the expression** — compile error ("unknown
variable"), per `buildEnv`'s contract (DR-C2).
- **`NewEngine` without a metamodel + a `condition:`** — construction error.
- **Concurrency** — one compiled `*Program`, concurrent writes; race detector on.
Note `Env` is not safe for concurrent declares (`env.go:117-118`) and `Bindings`
not for concurrent mutation (`bindings.go:37-39`) — compile once at load, fresh
`Bindings` per event.
- **Boundary values** — `days_between` at exactly 0 and negative (past-due).
- **`date_add` unit** — `day`/`week` accepted; `month`/`year` rejected in v1
with a clear error.

**Negative Tests:**

- Malformed `condition:` → **load-time error**, and specifically NOT the
silent-skip of `engine.go:61`. Highest-value negative test after AC5.
- Unknown function name / wrong arity / wrong argument type → compile error.
- Malformed RRULE → eval error (not compile — S5).
- `ExpectType` mismatch → compile error.
- `ExpectType(AnyType)` → option-validation error, not "accepts everything".

**Integration test approach:** end-to-end through the automation engine — a
metamodel declaring a `condition:`-bearing automation, a real write, assert the
action fired or did not. Plus an `appbuild`-level test that a bad `condition:`
fails project load.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| Binder/RecordType divergence → eval-time failures in production (S2) | **high** | AC5 round-trips every property type; this is the ticket's key test |
| `new`/`old` API cannot be changed after shipping (S1) | **high** | Decided now: `new`+`old`, no `entity`; documented in `docs/metamodel.md` |
| Constructor signature change ripples further than expected (C3) | medium | Verified exactly 2 production callers, both already error-returning |
| Move breaks an importer subtly | low | Type aliases + `go build` as the proof (L2); two-commit split (L1) |
| Silent-skip precedent copied into the new key | medium | AC7 + explicit negative test |
| Timezone skew between `today()` and date fns | medium | Same UTC convention; explicit boundary test |
| Two condition dialects confuse operators | medium | Distinct keys; `docs/metamodel.md` "which do I use" note covering the four-way vocabulary (M1) |
| Scope creep into `{{...}}` interpolation | medium | Explicitly out of scope |
| Arch-lint cycle from new edges | low | Verified: `predicatefns → {predicate,filter,metamodel}` and `automation → metamodel` already coexist; run `just arch-lint` early |
| Conflicts with parallel worktrees | low | Phase 1 is a large mechanical diff — confirm nothing in-flight touches `predicate`/`affordances` before starting |

**Effort: m**, at the top of the band. Phase 1 is mechanical bulk; phase 2–3 are
small; phase 4 plus the constructor change and validation-rule support is the
real work. If it grows, split phase 4 into its own ticket — phases 1–3 are
independently shippable and useful.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A at planning time: the automation creates it on the transition to `in-progress`)

**Documentation Impact:**

- [x] `docs/metamodel.md` — **required.** The `condition:` key on `on:` (and on
validation rules), the host-function set (`today`, `date_add`, `days_between`,
`rrule_next`, `match`, `regex`, `fuzzy`, `contains`), the `new`/`old` variables,
and guidance on `when:` (filter, data-shaping) vs `condition:` (expression,
computed). Document the four-way "condition/when/check/where" vocabulary (M1).
- [x] `CLAUDE.md` — **required.** `expr` is the general typed engine, `predicate`
its Bool-typed facade; neither may import `internal/metamodel`.
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command changes)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI changes)
- [x] ~~`README.md`~~ (N/A: no project-level change)

This is a refactor **plus** a user-facing metamodel feature, so the enhancement
docs path applies to the `condition:` half.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-5KYLPZ, RR-SEJ9UU, RR-K6IIIW, RR-O2AV40,
RR-Y0N50P, RR-2I7JHJ, RR-FTEW47 — all `addressed` in this revision.

| ID | Finding | Severity | Resolution |
|---|---|---|---|
| RR-5KYLPZ | C1 — "zero call-site edits" incompatible with `Eval` returning `bool`; two importers type-assert `Value`, and one branch is a fail-closed security decision | critical | `Eval` keeps `(Value, error)`; Bool ergonomics via additive `EvalBool` |
| RR-SEJ9UU | C2 — `Program` must be an alias, else struct fields in 2 packages + `CompileAll`'s nil contract break | critical | `Program` is an alias; facade is aliases + 5 functions |
| RR-K6IIIW | C3 — load-time error not achievable: `NewEngineFromMetamodel` returns no error; metamodel never validates automations; arch-lint edge missing | critical | Constructor gains `error`; 2 callers threaded; arch-lint edges added; `NewEngine`-without-metamodel path defined |
| RR-O2AV40 | C4 — `ExpectType` can't just replace line 109: `equalsType` asymmetric, `AnyType` would reject everything | critical | Restricted to scalars, argument validated at `Compile` entry |
| RR-Y0N50P | S1 — no `new`/`old` distinction in a change-triggered subsystem | significant | Env declares `new` + `old`; `entity` deliberately not declared |
| RR-2I7JHJ | S2 — `EntityRecordType` has no congruent binder; would fail at eval in production | significant | New `BindEntity`; AC5 round-trips every property type |
| RR-FTEW47 | S5 — compile-time RRULE validation describes a mechanism that doesn't exist | significant | AC4 scoped to eval-time |

Handled inline in this revision without separate entities (structural, not
defects in the design): S3 — `lint.go` moves to `expr`, `doc.go` split; S4 —
test files + `testdata/` added to Files, `arch_test.go` duplicated, error prefix
kept as `"predicate: "`; M1–M5 — vocabulary documented, AND semantics + ordering
specified, AC1 test replaced by an arch rule, `date_add` restricted to day/week
for v1, concurrency verified.
