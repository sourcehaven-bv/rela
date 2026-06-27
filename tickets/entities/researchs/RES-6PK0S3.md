---
id: RES-6PK0S3
type: research
title: Should filters, views, automations, and search converge on one comparison evaluator?
summary: 'Keep two evaluators by design: filter.Match for data-filtering (dates/enums/lists/fuzzy), predicate for policy (ACL/affordances). Fix the lexicographic bug at its one real site (automation validation) via filter.Match; make search.MatchFilters ordered ops error rather than silently-lexicographic; defer predicate-convergence until the affordance resolver proves predicate on the read path.'
status: done
---

## Problem

The codebase has **four parallel comparison-evaluation paths**, two of which
compare numbers and dates lexicographically (`"10" < "9"` → wrong). Review
finding **C2** asks: rather than tactically patch the two buggy matchers, should
we converge on one evaluator — and if so, which?

The four paths:

1. **`filter.MatchValue` / `matchStringSimple`** (`internal/filter/filter.go`) — string-only, lexicographic `<,<=,>,>=`. **Buggy** for numeric/date ordering.
2. **`search.MatchFilters`** (`internal/search/filter.go`) — string-only, lexicographic `Gt/Lt/Gte/Lte`. Part of the **store search conformance contract** (`storetest/search.go`); lexicographic ordering *intentionally pinned* ("low > high lexicographically"). No application code populates `Query.Filters` (only `storetest`) — contract-tested but app-unused.
3. **`filter.Match` / `MatchAll`** (`internal/filter/match.go`) — **type-aware** via metamodel `PropertyDef`. Correct ordering. Used by views, cli list, lua query, validation.
4. **`internal/predicate`** — typed, sandboxed Lua-expression interpreter. Used for ACL policy; predicate-engine affordance resolver is the documented next step (`affordances.go:97`).

## Context

### Call-site map (from exhaustive survey)

| Matcher | Production call sites | Has metamodel+PropertyDef? |
|---|---|---|
| `filter.MatchValue` (buggy) | `automation/engine.go:343` (validation `matchSimple`); `dataentry/views.go:183` (`type` pseudo-prop only) | engine: **No**; views: yes but used only for synthetic `type` field with no PropertyDef |
| `search.MatchFilters` (buggy) | `search/index.go:60,93` — but **no app code sets `Query.Filters`**; only `storetest/search.go` | matcher takes no metamodel; `search` imports neither `filter` nor `metamodel` |
| `filter.Match` (correct) | `views.go:199`, `helpers.go:659`, `cli/list.go:102`, `lua/runtime.go:785`, `validation/validation.go:271,279` | **Yes — all 7 sites already supply both** |

The buggy matchers are **barely load-bearing**: `search.MatchFilters`'s ordered
ops have zero app callers; `filter.MatchValue`'s only non-trivial caller is
automation-validation, which lacks metamodel context. The lexicographic bug is
therefore mostly *latent* — real for any automation `when:`/validation rule
using `<`/`>` on an integer/date today, but small in surface.

### Semantic gap: filter vs predicate are NOT the same tool

**`filter` (Match) supports, `predicate` lacks:** date type (metamodel
date-format parsing → `time.Time` compare), enum/status/priority types,
list/multi-select matching, glob (`*`), regex (`=~`), fuzzy/trigram (`~`), and a
terse `key op value` string syntax.

**`predicate` supports, `filter` lacks:** boolean composition (`and`/`or`/`not`,
nesting), host functions (`has_role(...)`), compile-time type checking,
step/depth budgets, record navigation. But only `bool`/`number`/`string` scalars
— **no dates, enums, lists, glob, regex, fuzzy**, and `number` is float64 (lossy
past 2^53; no integer type).

So `filter` is a **per-property data-filter DSL** (rich value types + matching
modes, simple syntax); `predicate` is a **boolean policy-expression language**
(composition + functions, typed but scalar-only). Neither is a superset of the
other. predicate's float64 model is also a regression for rela's `integer` type
and has no date semantics.

### Constraints

- `internal/search` imports neither `internal/filter` nor `internal/metamodel` (arch boundary). Routing `search.MatchFilters` through `filter.Match` means a new import + threading a metamodel into `Query`/`MatchFilters`, plus mapping `search.FilterOp` → `filter.Operator` (distinct enums; `FilterContains/In/Exists/NotExists` have no 1:1 in `filter.Filter`).
- Changing `search.MatchFilters` ordered ops changes the **store search conformance contract** (`storetest` pins lexicographic). That's a contract decision, not a silent bugfix.
- `automation.Engine` holds no metamodel; making its validation type-aware means threading `*metamodel.Metamodel` into the engine and resolving `event.Entity.Type` → `EntityDef` → `PropertyDef`, and deciding behavior for unknown types (today it tolerates "no metamodel context").
- CLAUDE.md: ACL gates evaluate against declarative policy + graph; **Lua participates only at write time**. `predicate` is the read-safe expression engine (no I/O, budgeted) — which is *why* it backs ACL. That makes predicate attractive for read-path policy, but filters/views are data-shaping, not policy.

## Options

### Option A — Tactical: make the two buggy matchers type-aware via `filter.Match`, keep four-evaluator structure otherwise
- **Do:** (1) Thread metamodel into `automation.Engine` validation so it calls `filter.Match` (or a numeric-aware comparison) instead of `MatchValue`. (2) For `search.MatchFilters`, decide the contract (see Option D) — minimally, make ordered ops numeric-or-error. (3) Leave `filter.Match` and `predicate` as-is.
- **Pros:** Fixes the correctness bug at its real sites. No new dependency between subsystems beyond what's needed. predicate untouched.
- **Cons:** Still four evaluators. Doesn't reduce the conceptual surface. The automation-engine metamodel threading is non-trivial (constructor change + unknown-type policy).
- **Effort:** M (automation engine wiring is the bulk).

### Option B — Converge data-filtering on `filter.Match`; keep `predicate` for policy (HYBRID)
- **Do:** Establish `filter.Match` as *the* data-filter evaluator. Delete `filter.MatchValue`'s ordered-op bug by routing all real-property comparisons through `Match` (the `type` pseudo-prop keeps a trivial string compare or gets a synthetic string PropertyDef). Resolve `search.MatchFilters` per Option D. Keep `predicate` as the policy/affordance engine. Document the boundary: **filter = data-shaping (views, lists, automation conditions, search), predicate = policy (ACL, affordances).**
- **Pros:** Two evaluators with a *clear, defensible boundary* (data vs policy), each the right tool for its job — `filter` keeps dates/enums/lists/fuzzy/glob; `predicate` keeps composition/functions/sandboxing. Fixes the bug. Aligns with how they're already used. No float64/date regression.
- **Cons:** Two evaluators remain (but intentionally). Requires the automation-engine metamodel threading (same as A). Doesn't unify syntax (`key=value` filter strings vs Lua-ish predicates).
- **Effort:** M.

### Option C — Converge everything on `predicate` (one expression engine)
- **Do:** Migrate views/automation-conditions/search filters to `predicate` expressions; extend `predicate` with a date type, enum/list semantics, and glob/regex/fuzzy host functions to reach feature parity with `filter`.
- **Pros:** One evaluator, one syntax, compile-time typing everywhere, sandboxed + budgeted uniformly. Maximal long-term simplicity; aligns with ACL/affordance roadmap.
- **Cons:** **Largest, riskiest.** Requires adding date/enum/list/integer types + glob/regex/fuzzy to predicate (a significant language extension, partly undoing its deliberate minimalism). float64 numeric model needs an integer story for rela's `integer` type. Rewrites the filter *string syntax* users already use in views/CLI (`status=draft`) into predicate expressions — a user-facing migration. Big-bang risk across views, CLI, lua, validation, search. The deferred affordance-resolver ticket would also need to land first to validate predicate on the read path at scale.
- **Effort:** XL (multi-PR, user-facing syntax migration, language extension).

### Option D — (orthogonal, applies under A/B) Resolve `search.MatchFilters` deliberately
Three sub-choices for the contract-tested-but-app-unused ordered ops:
- **D1 — Numeric-aware:** thread metamodel into search and delegate to `filter.Match`. Highest cost (new imports, op-enum mapping, Query plumbing) for a path no app uses.
- **D2 — Make ordered ops error/unsupported:** keep eq/contains/in/exists; `Gt/Lt/Gte/Lte` return an explicit "unsupported on string search filters" rather than silently-lexicographic. Update `storetest`. Removes the foot-gun cheaply, honest about capability.
- **D3 — Leave as-is, documented:** keep lexicographic, document it as "byte-order only; use `filter.Match` for typed comparison." Zero code; relies on no app ever using ordered `Query.Filters` wrongly.

## Recommendation

**Option B (hybrid: `filter.Match` for data-filtering, `predicate` for policy),
with Option D2 for search, sequenced to fix the correctness bug first without a
big-bang.**

Rationale:
- `filter` and `predicate` are genuinely different tools (data-shaping DSL with dates/enums/lists/fuzzy vs scalar-typed boolean policy language with composition/functions). Forcing them into one (Option C) means either crippling `filter`'s rich value/match types or bloating `predicate` with date/enum/glob/regex and an integer model — undoing the minimalism that makes predicate safe and auditable for ACL. The boundary "data vs policy" is real and worth keeping.
- The correctness bug lives almost entirely in **one place that matters** (automation validation). Fixing *that* is the high-value move; `search.MatchFilters`'s ordered ops have no app caller, so D2 (make them error rather than silently-wrong) closes the foot-gun without the cost of D1's cross-subsystem plumbing.
- Convergence-on-predicate (C) should not be ruled out forever, but it should be **driven by the affordance-resolver ticket** proving predicate on the read path first — not by the matcher bug. Note that as a future direction; don't build it now.

**Sequencing (each shippable, low-risk):**
1. **Fix the real bug:** thread `*metamodel.Metamodel` into `automation.Engine` and route validation `when:`/`then:` comparisons through `filter.Match` (numeric/date-correct). Behavior change is intended and scoped to automation/validation rules using `<`/`>` on integer/date — call it out in the PR. *(This is the C2 correctness fix.)*
2. **Close the search foot-gun (D2):** make `search.MatchFilters` ordered ops return an explicit unsupported-error; update the `storetest` conformance cases + comments to match. Small, isolated.
3. **Tidy `filter.MatchValue`:** keep it only for the `type` pseudo-property (or give that a synthetic string PropertyDef and delete `MatchValue`); document that all typed comparison goes through `filter.Match`.
4. **Document the boundary** in CLAUDE.md / package docs: filter = data-filtering, predicate = policy expressions; new comparison code picks by that axis.
5. **(Future, separate)** revisit predicate convergence once the affordance-resolver ships and predicate is proven on the read path.

**Tradeoff accepted:** we keep two evaluators rather than one, in exchange for
each staying the right tool for its domain and avoiding a user-facing
filter-syntax migration and a predicate language-bloat. The
lexicographic→numeric change in step 1 is a real (intended) behavior change for
existing rules that relied on string ordering.
