---
id: PLAN-LOXEMN
type: planning-checklist
title: 'Planning: Worlds: metamodel declaration, resolver, pushdown, selection (Step 2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

TKT-WAV8XP is Step 2 of FEAT-9CD2MX and ships as a **4-PR stack** on top of
TKT-DOFYR1. This checklist covers the whole ticket; **PR-A is the unit being
implemented first**.

A world is a metamodel-declared resolution function `resolve(world, entity) →
at most one state` (the *prime*), from three rules (design doc §4.1):

1. type declares no pointers → contributes its default state in every world;
2. type declares the pointer the world selects → that state is the prime; if
   the state does not exist, the entity **contributes nothing** (existence in
   a world *is* the publication bit);
3. type declares pointers but none the world selects → the world's mandatory
   `otherwise: exclude | default`.

IN SCOPE (whole ticket):

- `entities.<type>.pointers` and top-level `worlds:` metamodel declarations.
- A compiler (`internal/worlds`) producing a metamodel-free `store.WorldScope`.
- Load-time validation, with **mandatory `otherwise:`** (reject, never default).
- `World` scope on **both** `store.EntityQuery` and `store.GraphQuery`,
  evaluated in shared fs/mem code and pushed down to pgstore SQL.
- A world read decorator below the visibility gate (`visibility ∘ world`),
  with provenance on the prime.
- World-selection by **wiring-site binding ONLY** — a surface is constructed
  over its world and has no world parameter at all (the DEC-ZBI39P stance:
  structurally incapable, not "defaults to published").
- `analysis.CheckStates` subtracting the declared pointer set from its
  `undeclared-pointer` finding.

OUT OF SCOPE:

- **Per-world read grants / state-shaped write grants** — Step 3, TKT-DN37J2.
  **[Q10 ANSWERED]** `?world=` / `--world` is NOT exposed in Step 2 at all.
  Request-level selection lands in Step 3 TOGETHER with its grant check —
  Jeroen: "I will not ship partial stuff." A selectable-but-ungated world
  parameter is exactly partial, so Step 2 ships no parameter rather than a
  parameter that refuses. Owed work to record on TKT-DN37J2.
- **The copy kernel / promote / CoW** — Step 4, §9.
- **Per-world search INDEXING and cross-world grouping** — Step 5, TKT-9KZGJO.
  The Step-1 deliberate skip (indexing observers ignore non-default-pointer
  events) stays exactly as it is; Step 2 must not quietly start indexing states.
  **[design review, RR-7FOWDB] CORRECTION — this does NOT mean search needs no
  world work in Step 2.** That reading is false and would ship a bypass: the
  SEARCH READ PATH has its own SQL builder (`pgstore/visiblesearch.go:239-241`)
  and its own `AllowAll` short-circuit (`search/visible.go:258-262`), so a
  PR-D `world(published)` surface would get a working search box returning
  default-world hits — drafts. **IN SCOPE for Step 2:** search must REFUSE a
  non-default world (`search.ErrScope`) rather than silently serve the default
  one, so the wiring cannot be built wrong. Awaiting architect ruling.
- **World templates (§4.5, axis fallback + `for_each`)** — explicitly tentative
  in the design doc, and **not trivially free**: both mechanisms need the axis
  concept, which does not exist. PR-A deliberately leaves *no half-built hook*.
- **Pointer/axis re-keying** — rides the general data-migration system
  (DEC-0VGTF3 / FEAT-T3EF5A). v1 ships DETECTION only.
- `RelationQuery` world scope — see Risks (architect Q4, held).

**Acceptance Criteria:**

Whole-ticket, each with its test scenario. AC1–AC5 are PR-A.

1. **A metamodel with no `worlds:` block compiles to the zero `WorldScope`.**
   Test: load a fixture without `worlds:`/`pointers:`, assert
   `worlds.Compile(m)` yields a map whose default entry satisfies
   `IsDefaultWorld()` and whose `ByType` is empty (no allocation for a
   pointerless project).
2. **`otherwise:` is mandatory.** Test: a `worlds:` block declaring a world
   without `otherwise:` fails metamodel load with an error naming the world
   and the two allowed values. Explicitly NOT defaulted — a silent fallback is
   the leak this feature exists to prevent (§4.1).
3. **`select` / `overrides` may only name pointers the type declares.** Test:
   a world selecting `published` for a type declaring only `draft` fails load,
   with an error naming world, type, and pointer.
4. **At most one `default: true` pointer per type**, and pointer names obey
   the `entity.ParsePointer` grammar. Test: two defaults → load error; a
   pointer named `Draft` / `a--b` / `1x` → compile error naming file, type,
   offending name, and the grammar.
5. **`analyze states` subtracts declared pointers, PER TYPE.** Test: a project
   declaring `draft` with rows stored under `draft` and `legacy` reports
   `undeclared-pointer` for `legacy` only.
   **[design review, RR-E1C216]** The declared set is a map `type → set`, not
   a set — `pointers:` is per entity type. A flat subtraction would suppress
   `POLICY-1@draft` when only `page` declares `draft`, hiding exactly the
   stranded-data class that matters most (a pointerless type contributes its
   default state in every world, so that row is reachable through NO world).
   `analysis/states.go:104-169` currently aggregates `byPointer` globally;
   `stateRow.typ` (`:46-49`) already carries what is needed.
   **Decision for the implementer, not to be made at the keyboard:** filter
   per row at `states.go:119` with `if declared[st.typ][st.pointer]
   { continue }`, keeping the pointer-keyed headline finding and the existing
   `Subject` JSON contract. Additional negative test: *a pointer declared on
   type A but stored on type B still reports.* For a `headless-family` row,
   `st.typ` is the state's own type and may match no declaration — it still
   reports, which is correct.
   **[design review, RR-KNDLGR]** AC5 reads the declared set from
   `*metamodel.Metamodel` (already held as `analysis.Deps.Meta`), NOT from the
   compiled `store.WorldScope`. arch-lint already permits `analysis →
   metamodel`, so no arch-lint edit; and the check must keep working on a
   project whose `worlds:` block is malformed.
6. **The zero `WorldScope` is byte-identical to today.** Test (PR-B,
   `storetest`): every existing query shape returns the same rows with a zero
   `World` as it does today.
7. **At most one prime per (world, entity).** Test (PR-B): an entity holding
   both `review` and `published` under `select: [review, published]` yields
   exactly one row, the `review` one.
8. **`otherwise: exclude` contributes nothing; `otherwise: default` resolves
   to the default state.** Test (PR-B) both arms.
9. **`AllStates` + `World` together is an error.** Test (PR-B): all three
   backends return `store.ErrInvalidQuery`.
10. **The pushdown path and the decorator path agree.** Test (PR-D): for the
    same (world, principal), `listPushdown` and the decorator return the same
    set — the `acl/policy_parity_test.go` discipline.
11. **Resolution is principal-independent (guard rule 1).** Test (PR-D):
    given `select: [review, published]`, a principal denied `review` gets
    NOTHING — never the `published` face. Plus a package-scan structural guard
    (see Test Plan).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~
      (N/A: the design of record is `.ignored/pointer-design.md` v2 §4, already
      architect-reviewed; a 10-question survey was run instead and 8 were
      answered — see below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — design of record is `.ignored/pointer-design.md` §4;
the planning survey is `.ignored/wavx-plan.md` (F1–F5 reality check, the
compiled-world proposal, the 10 architect questions and 8 answers).

**Existing Solutions:**

*Libraries:* none applicable. The compiled form is a small ranked-preference
table; the evaluation is `DISTINCT ON` in SQL and an argmin in Go. Any
expression/rules engine would be strictly worse: the resolution function is
deliberately non-Turing-complete (substitution only, no conditionals — the
§4.5 "Helm cautionary tale"), and the whole point is that it PUSHES DOWN into
SQL, which a general engine cannot.

*Patterns in this codebase, all reused rather than reinvented:*

- **`internal/visibility` / DEC-ZBI39P** (`visibility.go:1-45`) — the read
  decorator above an ungated base, injected at the wiring site. The world
  resolver is the same shape, sitting one layer lower.
- **ACL read-query pushdown** (`internal/acl/readquery.go:27-95`,
  `internal/visibility/pushdown.go:56-94`) — the precedent for "express the
  scope as a store query, don't filter per row", including the three-valued
  `ReadQueryResult` and the fail-closed discipline. §4.3 explicitly invokes
  it: *"same argument, same solution shape"*.
- **Optional store capabilities type-asserted at the wiring site**
  (`store.Formatter`, `HistoryReader`, `DerivedSchemaReconciler` —
  `store/derivedschema.go:8-19`) — the precedent for adding backend ability
  without widening `store.Store`.
- **`storetest` conformance-before-second-backend** (`storetest/states.go:15-22`,
  which exists for exactly this reason) — §14 names it the drift guard.
- **`validateRelationScope`** (`loader.go:649-665`) — the direct precedent for
  a DOFYR1-era declaration validated at load: sorted-key iteration, collected
  (not fail-fast) errors, message naming the subject and the allowed values.
- **`analysis.CheckStates`'s decide-after-the-scan discipline**
  (`analysis/states.go:57-62`) — no backend documents `AllStates` iteration
  order, so per-family decisions must not be made mid-stream. This applies
  verbatim to fallback resolution.
- **`acl/ceilingguard_test.go`** — a package-scanning test with an exemption
  list, the model for the principal-independence structural guard.

*Prior art consulted in the ticket graph:* FEAT-9CD2MX, TKT-DOFYR1 (Step 1,
done), DEC-ZBI39P, DEC-0VGTF3, DEC-PEYCJZ, DEC-8UIL0.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*The central design question — what does a world compile to?*

**Not a row predicate.** `pointer IN ('review','published')` returns TWO rows
for an entity holding both, breaking the at-most-one-prime invariant that
§4.2 says everything leans on. Nor can a per-row predicate express ordered
preference (per-family argmin) or rule 2's "contributes nothing" (a row-
*absence* fact). A world compiles to a **per-type ranked coordinate list plus
a fallback verdict**, and the store evaluates an argmin per state family.

```go
// internal/store/world.go  (in `store` so fs/mem/pg AND visibility may name
// it; `visibility` may not import `metamodel` under arch-lint)

type WorldScope struct { byType map[string]TypeResolution } // UNEXPORTED
type TypeResolution struct { Chain []entity.Pointer; Fallback Fallback }
type Fallback int
const ( FallbackExclude Fallback = iota; FallbackDefaultState )

func (w WorldScope) IsDefaultWorld() bool { return len(w.byType) == 0 }

// For carries the TWO-VALUED answer; see [design review] below.
func (w WorldScope) For(entityType string) (TypeResolution, bool)
```

**[design review, RR-CZN30X]** The map field is **unexported and never
indexed directly**. `w.byType["unknown"]` returns the zero `TypeResolution`
= `{nil, FallbackExclude}`, which reads as "exclude this whole type" — the
exact OPPOSITE of what absence means here (rule 1: contribute the default
state). Absence and zero-value meaning opposite things through one
map-index expression is an API trap that no godoc fixes; it would produce
exactly one bug, in a backend, silently excluding a type from a world.
Backends go through `For` (or a `Candidates` method), and `internal/worlds`
constructs via a constructor. Free to fix now — the type does not exist yet.

Architect-decided details (Q1, Q2, Q3, Q6, Q7, Q8, Q9):

- `FallbackExclude` is the **zero value** — a half-built resolution hides
  content rather than leaking a draft. The two adjacent zero values mean
  different things on purpose (zero `WorldScope` = the *total* default world;
  zero `Fallback` = *exclusion*), so both godocs state the contrast
  explicitly. No `FallbackUnset`: a validation branch that can be forgotten is
  worse than a zero value that fails safe.
- `otherwise: default` compiles to a **Fallback, not an appended chain
  element** — chain-suffix is simpler SQL but destroys provenance, makes
  `analyze` unable to report "fell back for N entities", and conflates
  rule-3-default with rule-2-on-a-declared-default-pointer.
- Rule 1 (pointerless types) is expressed by **absence from `ByType`**, so a
  mixed graph costs zero per-type work.

*PR-A concretely (the unit being built now):*

1. `metamodel`: `EntityDef.Pointers map[string]PointerDef` (`default: true`),
   `Metamodel.Worlds map[string]WorldDef` (`select` string-or-list,
   `overrides map[string]…`, `otherwise`, `edits` parsed-but-unused).
   Add `"worlds"` to `validTopLevelKeys` (`loader.go:16-28`) — `checkUnknownKeys`
   rejects it otherwise.
2. `metamodel` structural validation in the `validateRelationScope` mold,
   appended to `validate()` (`loader.go:243`): mandatory `otherwise:`,
   `select`/`overrides` naming declared pointers, at most one `default: true`
   per type, chain dedup after resolution, world name uniqueness, `default`
   reserved as a world name. All **collected**, not fail-fast, so an operator
   sees every problem at once.
3. `internal/worlds` (new package; arch-lint `mayDependOn: [entity, metamodel,
   store]`): `Compile(*metamodel.Metamodel) (map[string]store.WorldScope, error)`
   plus `Default()`. **Pointer-grammar validation lives here** (Q6(b)), because
   `metamodel` may not import `entity` — with the requirement that the error
   names file, type, offending pointer name, and the grammar, i.e. is as good
   as a loader error, and that it fires at **startup** (compile runs during
   appbuild/`SharedBase`), never lazily at runtime.
4. `store.WorldScope` + `TypeResolution` + `Fallback` types only — nothing
   consumes them yet.
5. `analysis.CheckStates`: subtract the declared set from `undeclared-pointer`
   (`analysis/states.go:14-27` already promises this in its doc comment),
   per `(type, pointer)` and reading `*metamodel.Metamodel` — see AC5.
6. **[design review, RR-LLLBQY]** `internal/appbuild/appbuild.go:899-917`
   (`warnUndeclaredPointers`). Its two-COUNT probe warns that the store holds
   rows "no metamodel declaration accounts for", and its doc comment states
   the Step-1 premise that PR-A retires. Left alone, every boot of an adopting
   project warns about perfectly declared drafts — warning fatigue on the one
   diagnostic that surfaces genuinely stranded data. **Fix (a):** gate the
   warning on "no type declares pointers", preserving Step-1 behavior for
   pointerless projects and going silent for adopters. Update the doc comment.
7. **[design review, RR-CGRV0X]** `validateWorlds` also cross-checks
   `metamodel.RelationScope`: a `content`-scoped relation's endpoints should be
   types that actually declare pointers. Load-time only, cheap, does NOT
   pre-commit Q4 — and it becomes a migration once `worlds:` blocks exist in
   the wild.

*PR-B* — `World` on `EntityQuery` AND `GraphQuery`; `storeutil.WorldPrimes`
(buffer per family, decide at end-of-family) plus folding the duplicated
`fsstore.matchEntityQuery` / `memstore.matchEntityQuery` into a shared
`storeutil.MatchEntityQuery`; `storetest.RunWorldTests`; fs+mem implement;
pgstore **rejects a non-zero `World` loudly** (Q8) until PR-C.

**[design review, RR-MNOBJK] — the harness forces a mechanism the plan must
name.** `storetest.RunAll` (`storetest.go:150-170`) is flat and unconditional
apart from `Capabilities`; all three backends call it identically and the
postgres CI job sets `RELA_TEST_DATABASE_REQUIRED: "1"` so it cannot skip.
Adding `RunWorldTests` therefore runs it against pgstore, forcing one suite to
accept two contradictory outcomes — which destroys the §14 conformance
discipline. Mechanism (the DOFYR1 precedent):
(1) add `Capabilities.Worlds bool`, gate `RunWorldTests` on it; fs/mem set it
in PR-B, pgstore in PR-C, **and PR-C removes the flag** so it cannot become a
permanent opt-out (the flag exists for exactly one commit window);
(2) add a small UNCONDITIONAL `RunWorldRejectionTests` asserting a
capability-less backend returns a specific sentinel rather than a wrong answer
— that is the loud-rejection assertion, and it belongs in the always-on suite;
(3) pin error precedence — AC9's `ErrInvalidQuery` check must run BEFORE the
unsupported-world rejection, or the two sentinels collide.

**[design review, RR-CUUZ9Z] — the single-entity path.** `Get` resolves the
chain via `GetEntityState` per coordinate (chains are 1–3; the common case is
one call), applies the fallback verdict, and returns not-found under
`FallbackExclude` — **indistinguishable from a genuine miss and from an ACL
denial**. Decide here, not in PR-D, whether `store.EntityReader` gains a
world-aware get or resolution lives entirely in the decorator.

*PR-C* — pgstore SQL. `DISTINCT ON (e.id) … ORDER BY e.id, c.rank` against
`unnest($chain) WITH ORDINALITY` expresses at-most-one-prime in the planner;
`FallbackDefaultState` adds a `UNION ALL … NOT EXISTS` arm, `FallbackExclude`
omits it. All **four** sites (`entityWhere`, `buildGraphQuerySQL`,
`buildMatchingIDsSQL`, `GraphCount`'s `total`) — F1/F5 — **plus
`visiblesearch.go:241`** (RR-7FOWDB: an independent SQL builder the four-site
count missed). The recursive-CTE seeds (`graphquery.go:326`) are the
**identity anchor** and stay un-worlded (Q5 — §8.3: role/containment climbing
is world-insensitive).

**[design review, RR-NJSCP5] — the comment taxonomy is THREE-valued**, and the
literal count is ~20 across nine files, not four. A binary taxonomy mislabels
a third category and invites a later "fix" that breaks id allocation:

- `// identity anchor` — e.g. `graphquery.go:326`.
- `// default world (world-scoped in PR-C)` — the four query sites + search.
- `// family-scoped write path — deliberately pointer-unaware, do NOT world`
  — `entity.go:173-174` (`HighestID`; states share their family's number, so
  worlding it would let two entities get the same sequential id),
  `derivedschema.go:240-246,306,322` (`unique:` is a natural-key rule over the
  default state), `entity.go:296` (the family-type `FOR SHARE` check), and the
  versioning/sync infrastructure in `manifest.go`, `sweep.go`, `purge.go`,
  `relation.go`, `relation_version.go`.

Enumerate via `grep -rn "pointer = ''\|from_pointer = ''"
internal/store/pgstore/` so PR-C's author is not discovering scope mid-PR.

*PR-D* — the decorator (`visibility ∘ world`, world innermost so the prime is
picked before the gate is consulted), `Resolved{Entity, Pointer, Fallback}`
provenance as a **sibling struct** (Q7 — never a field on `entity.Entity`,
which is one `UpdateEntity` from being persisted), wiring in
`appbuild`/`SharedBase`, selection plumbing, the parity test and the
principal-independence guards.

**[design review, RR-EHER1V] — counts must carry the world.**
`gatedGraphReader.CountEntities` (`appbuild.go:528-535`) deliberately hits the
RAW store, justified at `:495` by "a count is STRUCTURAL… an aggregate tally of
a type the metamodel already publishes is not [a secret]". That is right for
ACL and **wrong for worlds**: §4.1 makes existence in a world the publication
bit, so an unscoped tally tells a published-world surface how many unpublished
drafts exist. The existing comment ends "If either judgement changes, this is
the one type to fix" — this is that change. Fix: the world rides on
`EntityQuery` and `CountEntities` passes it through to the raw store, which
still applies the WORLD while skipping only the ACL gate (which is what the
comment actually means). Two lines, but silence here reads as "counts are
fine". Sidebar counts (`dataentry/views_handler.go:330`) go through
`GraphQuery` and inherit `World` for free.

**[design review, RR-CUUZ9Z]** PR-D gains an acceptance criterion for
single-entity resolution: a `world(published)` reader's `Get` must not return
the default/draft face. AC10's parity test must cover the `Get` path, not only
list paths.

#### Relation reads under a world (Q4 ANSWERED — decorator-layer scope dispatch)

`RelationQuery` gets **no world**, and there is **no new store-contract
surface**. DOFYR1's contract stands frozen and every `storetest` pin (nil =
unfiltered, `&zero` = default-tail-only, tail is part of relation identity)
stays valid.

*Why a world on `RelationQuery` is impossible, not merely unattractive.*
"Edges of X in world W" is the scope-conditioned disjunction `(tail = prime
AND content-scoped) OR (tail = '' AND identity-scoped)`. Three facts from the
tree make that unevaluable in the store: relation SCOPE is metamodel-only
(zero references to `RelationScope` outside `internal/metamodel`, verified);
`store.WorldScope` is deliberately metamodel-free; and `RelationQuery`'s
fields are AND-ed conjuncts with no disjunction primitive, while
`FromPointer` has no "this-state-or-identity" value. Worse, identity edges
and default-state content edges are **stored identically** — both
`from_pointer = ''`, no discriminator (`fsstore/relation.go:132-135`,
`memstore.go:801`, `pgstore/relation.go:135-158`) — so no two-query merge can
separate them either.

*The decorator does the dispatch*, because it is the one layer that
legitimately holds BOTH the metamodel (scope) and the world (prime):
identity-scoped relation types are queried with a **nil** tail;
content-scoped types with the **prime's pointer**; results merged. One
resolution site, no second argmin, no store change.

*Placement (determined, no arch-lint change needed).* It lives in
`internal/worlds`, whose arch-lint deps are already exactly `entity +
metamodel + store`. It cannot live in `internal/visibility`, which may depend
on `{acl, affordances, entity, principal, store, tracer}` — **no metamodel**.

*Mandatory follow-through in PR-D:*

1. **The fallback trap, explicitly handled and tested.** When
   `otherwise: default` fires, the prime's pointer IS the zero pointer —
   which as a `FromPointer` value means "default-tail ONLY", a **different
   filter from nil**. So a content-scoped query for a fallback prime is
   correctly `&zero`, while an identity-scoped query must stay `nil`. Pin
   both in a test whose name says so.
2. **The omission must be unrepresentable.** The world-resolved surface hands
   out the relation-reading capability carrying the dispatch; a consumer must
   not be able to issue a raw nil-tail query through it. A
   forget-and-fail-open shape is unacceptable for something that gates role
   conferral.
3. **Acceptance cases are the two real Step-2 consumers**: the tracer's seven
   nil-tail reads (`tracer.go:122,134,176,251,257,311`) and
   `acl/storegraph.go:55`'s role-conferral walk. Pin that a world-resolved
   tracer does NOT see a non-prime state's content edges, and that identity
   edges remain visible from every face — the latter is what TKT-DN37J2
   requires of identity-scoped role relations.

**Alternatives considered and rejected:**

| Alternative | Why rejected |
| --- | --- |
| World as a `(type, pointer)` set predicate | Breaks at-most-one-prime; cannot express ordered fallback or "contributes nothing". §2.1 of the survey. |
| `otherwise: default` as an appended chain element | Simpler SQL, but destroys provenance and conflates two distinct rules (Q2). |
| Per-row Go filter above the store | Explicitly forbidden by §4.3 — recreates the problem ACL pushdown solved. |
| World scope on `EntityQuery` only (as the design doc says) | **F1/F5**: `listPushdown` swaps to `GraphQuery`, and its `AllowAll` branch stays on `EntityQuery` — a world on one path only is fail-open for the *privileged* principal. |
| World resolution inside the recursive CTE walks | Would make an entity's ancestors depend on the reader's world; §8.3 rejects it (Q5). |
| Compiler inside `internal/metamodel` | Would give `metamodel` — imported nearly everywhere — a `store` dependency (Q9). |
| Provenance as a field on `entity.Entity` | Persist hazard (Q7). |
| pgstore naive fallback in PR-B | "Temporarily slow" silently becomes permanent; §4.3 forbids the per-row path in the backend that has scale (Q8). |

**Files to modify (PR-A):**

- `internal/metamodel/types.go` — `Metamodel.Worlds`, `EntityDef.Pointers`,
  `WorldDef`, `PointerDef`, `Otherwise` type + `IsValid()`.
- `internal/metamodel/loader.go` — `validTopLevelKeys` (+`worlds`),
  `validate()` wiring, `validateWorlds` / `validatePointers`.
- `internal/metamodel/worlds_test.go` (new), fixtures under `testdata/`.
- `internal/worlds/worlds.go` + `compile.go` + tests (new package).
- `internal/store/world.go` (new) — `WorldScope`, `TypeResolution`, `Fallback`.
- `internal/analysis/states.go` — declared-set subtraction; `states_test.go`.
- `.go-arch-lint.yml` — the `worlds` component + deps entry.
- `docs/metamodel.md` — `pointers:` / `worlds:` reference (see Doc Planning).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
| --- | --- | --- | --- |
| `worlds:` block | `schema.yaml` — operator-authored, in-repo | Structural checks in `metamodel` (mandatory `otherwise:`, chain references a declared pointer, unique names, `default` reserved) | **Hard load error**, collected so all problems surface at once |
| pointer names | `schema.yaml` | `entity.ParsePointer` **allowlist grammar** `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`, applied in `internal/worlds` (Q6b) | Compile error at startup naming file/type/name/grammar |
| ~~`?world=` / `--world`~~ | — | **[Q10 ANSWERED] Not an input in Step 2.** The parameter is not exposed; world selection is wiring-bound only. It becomes an input in Step 3 with its grant check, where the allowlist (must name a compiled world) and fail-closed rule apply | n/a in Step 2 |
| stored `pointer` column/filename | storage | Already validated by the DOFYR1 codec; the store equality-matches and never inspects | unchanged |

Note the CLAUDE.md rule: **the configuration is not a secret.** World names,
pointer names and `otherwise:` values are operator-authored config in a
routinely-public repo. Code must NOT contort to conceal them — a 403 naming
an undeclared world is the right answer for a config-declared capability.
What IS secret is entity/relation content and entity *existence*, which is
exactly what rule 2 ("contributes nothing") and the row gate protect.

**Security-Sensitive Operations:**

1. **Fallback resolution is the feature's core security surface.** A world
   quietly showing a draft face where an operator asked for published is the
   leak this ticket exists to prevent. Protections: `otherwise:` is mandatory
   and load-validated (never defaulted); `FallbackExclude` is the zero value;
   `analyze` reports when fallback fired.
2. **Guard rule 1 — resolution is principal-independent.** ACL must never
   participate in fallback: serving the next face down when the prime is
   hidden is an existence oracle ("being served the fallback tells you a
   hidden state exists above it"). Protections: decorator order
   `visibility ∘ world` with world **innermost**, so the prime is chosen
   before the gate is consulted; the resolver's constructor takes no
   `RowGate` / `acl.*` / `principal` import; a behavioural test AND a
   package-scan structural guard.
3. **Pushdown/decorator divergence is a fail-open class (F1/F5).** A world
   carried on `EntityQuery` but not `GraphQuery` leaves the `AllowAll`
   principal — the privileged one — unscoped. Protection: the scope lands on
   both, at all four pgstore sites, with a parity test.
4. **No SQL is built from pointer or world values.** Chains travel as bound
   array parameters (`unnest($1::text[]) WITH ORDINALITY`); ranking is
   positional. The store keeps DOFYR1's invariant: equality-match only, never
   inspect.
5. **Errors leak nothing sensitive.** Load/compile errors name only
   operator-authored config (permitted, per the rule above). Resolution errors
   must not distinguish "excluded by the world" from "denied by ACL" from
   "does not exist" on a read-out path — all three are the uniform miss.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1–AC5 map to `internal/metamodel` loader tests and
`internal/worlds` compiler tests (table-driven with `t.Run`, per project
convention) plus an `internal/analysis` test for AC5. AC6–AC9 map to
`storetest.RunWorldTests`, which every backend runs — that is the integration
layer for the store contract, and per §14 it lands in PR-B **before** pgstore
implements in PR-C. AC10–AC11 map to `internal/visibility` tests plus the
package-scan guard.

**Integration approach beyond unit tests:**

- `storetest.RunWorldTests` executed by fs, mem, and (from PR-C) pg — the
  cross-backend drift guard §14 mandates. pg runs DB-gated on
  `RELA_TEST_DATABASE_URL` via `just test-postgres`.
- `graphquery_explain_test.go` extended in PR-C: assert the world arm does not
  silently degrade to a sequential scan on the indexed path, or record
  explicitly that it does and why that is acceptable.
- The decorator/pushdown parity test (PR-D) is an integration test by nature:
  it runs both code paths against one store and one policy.
- A **structural guard** (PR-D), in the `acl/ceilingguard_test.go` spirit:
  scan the resolver package and fail if it imports `acl`/`principal` or if a
  constructor accepts a `RowGate`. The behavioural test alone does not survive
  a refactor; this does.

**Edge Cases:**

| Case | Expected |
| --- | --- |
| No `worlds:` block at all | Zero `WorldScope`; `ByType` empty (no allocation); every existing query byte-identical |
| No `pointers:` on any type | Same as above; rule 1 for every type |
| Mixed graph (pointerless `ticket` beside pointered `page`) | `ticket` absent from `ByType`, contributes in every world; no special handling |
| Entity holds BOTH chain coordinates | Exactly one row, the higher-ranked one (AC7) |
| Entity holds NEITHER chain coordinate, `otherwise: exclude` | Contributes nothing |
| Entity holds NEITHER, `otherwise: default`, and has no default row | Contributes nothing (headless family — DOFYR1's `headless-family` finding); must not error |
| Empty `select: []` with an `overrides` for the type | Override wins; type takes the override chain |
| `overrides` naming a type with no pointers | Load error (rule 1 types have no chain to override) |
| Chain with a repeated coordinate (`[draft, draft]`) | Dedup after resolution (§4.5's dedup rule, applied even without templates) |
| Single-element chain (the common case) | Must not emit the UNION arm when `otherwise: exclude` |
| Cursor/pagination across a family boundary | Families are contiguous under `(id, pointer)` ordering, so the buffer window is ONE family, not the result set — pinned as a conformance case |
| `AllStates` + `World` set together | `store.ErrInvalidQuery` from all three backends (AC9) |
| Very long chain / many types | Bounded by declared pointers; no unbounded growth (§7's "bounded-and-declared" argument) |
| Unicode / uppercase / `--` in a pointer name | Rejected by the grammar (`ParsePointer` already rejects `--` because the pointer serializes into the relation-key FROM slot) |
| World named `default` | Reserved; load error |

**Negative Tests:**

- World without `otherwise:` → load error naming the world and both allowed
  values. (The single most important negative test in this ticket.)
- World `select`ing an undeclared pointer → load error naming world, type,
  pointer.
- `overrides` naming an unknown entity type → load error.
- Two pointers with `default: true` on one type → load error.
- Bad pointer grammar → compile error naming file, type, name, grammar, at
  startup.
- `worlds:` present on an old binary → `checkUnknownKeys` reports
  `unknown key "worlds"` with the valid-key list. Acceptable and loud (the
  DOFYR1 forward-compat stance: newer data fails loudly on an older binary
  rather than being misread).
- Non-zero `World` against pgstore before PR-C → loud rejection, not a slow
  path (Q8).
- Principal denied the prime → **nothing**, never the next face (AC11).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort:** xl for the ticket (unchanged); PR-A alone is ~m.

**Risks:**

| Risk | Severity | Mitigation |
| --- | --- | --- |
| **Pushdown/decorator divergence** (F1/F5) — the design doc named only `EntityQuery`; the real ACL path uses `GraphQuery`, and `AllowAll` uses neither | High (fail-open for the privileged principal) | Scope on both query types; all four pgstore sites; explicit parity test (AC10). Recorded as a design-doc correction; architect is amending §4.3/§8.1 |
| ~~Q4~~ **ANSWERED** — `RelationQuery` gets NO world; scope dispatch happens at the decorator | Closed | See "Relation reads under a world" below. No store-contract change; DOFYR1's contract and every storetest pin stand frozen. |
| **[design review] Search read path is unscoped** (RR-7FOWDB) — a fifth SQL site and a second `AllowAll` bypass, both currently out of scope | **Critical** | Blocks PR-D at minimum; needs an architect ruling on whether Step 2 threads the world into the search seam or refuses non-default worlds there. Refusal is the cheap, fail-closed answer |
| **[design review] `Get` path unresolved** (RR-CUUZ9Z) — the plan is list-shaped; `GetEntity` is contractually the default state | **Critical** | Mechanism decided in PR-B (chain via `GetEntityState`, fallback verdict, uniform not-found); acceptance criterion in PR-D |
| **[design review] `storetest.RunAll` is unconditional** (RR-MNOBJK) — PR-B's "pg rejects loudly" cannot coexist with a shared conformance suite | Significant | `Capabilities.Worlds` gate + a separate unconditional rejection suite + pinned error precedence; PR-C removes the flag |
| **[design review] Boot warning goes false** (RR-LLLBQY) — `warnUndeclaredPointers` warns about declared drafts from PR-A onward | Significant | Gate on "no type declares pointers"; update the doc comment |
| ~~Q10~~ **ANSWERED** — no `?world=` in Step 2 | Closed | The parameter ships in Step 3 with its grant. PR-D shrinks accordingly: resolver + provenance + wiring-bound surfaces + guards + parity test, no request-level selection |
| **Two adjacent zero values with opposite meanings** (`WorldScope` zero = total default world; `Fallback` zero = exclusion) | Medium (comprehension trap) | Architect-decided: keep both, mitigate with naming + explicit contrasting godoc on each. No third state |
| **Four implementations of one scope** (fs, mem, pg-EntityQuery, pg-GraphQuery) — worse than the doc's "three", because `matchEntityQuery` is duplicated (F3) | Medium | `storeutil.WorldPrimes` + shared `MatchEntityQuery` collapses fs/mem to one; `storetest.RunWorldTests` lands BEFORE pg implements (§14's drift guard) |
| **`pointer = ''` spells two different concepts** (default world vs identity anchor) | Medium | PR-C required deliverable: disambiguating comment at every literal. Q5 settled: CTE seeds are the identity anchor and stay un-worlded |
| **Performance of the world query on a large type** — the join is `(type, pointer)`, the DISTINCT ON is `id`; existing indexes are `entities_type_idx` and PK `(id, pointer)` | Medium | Measure, don't speculate: `graphquery_explain_test.go` + `graphquery_bench_test.go` already exist as the harness. A new index is a decision made on evidence, not a speculative migration |
| **fs/mem buffering** — per-family candidate buffering during a list | Low | Only ever runs for a NON-default world; `IsDefaultWorld()` short-circuits to today's path. Bounded to one family by `(id, pointer)` contiguity |
| **Regression for pointerless projects** | Low but unacceptable if it happened | Zero-value fast path is structural, not incidental; AC1 + AC6 pin it; the pointerless project never allocates a map |
| **Scope creep into Steps 3/4/5** | Medium | Explicit out-of-scope list above; grants are a named seam only; §4.5 templates get no half-built hook |

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/metamodel.md` — **new `pointers:` and `worlds:` reference**: the
      three resolution rules, `select` chains, `overrides`, and especially
      that `otherwise:` is mandatory and why. The primary user-facing doc for
      this ticket.
- [x] `CLAUDE.md` — a short rule once the resolver lands (PR-D): reads address
      worlds, writes address states; world required in the interior; the
      resolver sits below the visibility gate and ACL never participates in
      fallback. Deferred to PR-D, not PR-A.
- [x] `docs/cli-reference.md` — only if `--world` ships in Step 2 (blocked on
      Q10 — so likely NOT in Step 2); the `analyze states` declared-set
      behavior does land here.
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI change in Step 2; world selection
      in the SPA arrives with Step 3's grants)
- [x] ~~`README.md`~~ (N/A: no project-level change)
- [x] `docs/postgres-backend.md` — a note on the world pushdown shape when
      PR-C lands.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-7FOWDB (critical), RR-CUUZ9Z (critical),
RR-MNOBJK, RR-E1C216, RR-LLLBQY, RR-EHER1V, RR-CGRV0X, RR-CZN30X
(significant), RR-NJSCP5, RR-KNDLGR (minor).

Ten findings, all verified against the tree (several design-doc and
survey citations were checked and held; three plan claims did not).

**Two criticals block implementation pending an architect ruling**, because
both change what Step 2 must CONTAIN, not merely how PR-A is written:

- **RR-7FOWDB — search is a fifth world-scope site and a second `AllowAll`
  bypass.** `pgstore/visiblesearch.go:239-241` is an INDEPENDENT SQL builder
  hardcoding `e.pointer = ''`; `search/visible.go:269` routes through
  `GraphQuery` (so it silently takes the zero World once PR-B lands) and
  `:258-262` reproduces the F5 `AllowAll` short-circuit in a second package.
  This **contradicts the approved OUT-OF-SCOPE line** "per-world search
  indexing is Step 5; the Step-1 skip stays as it is" — that statement is
  true of the INDEX but smuggles in the false claim that search needs no
  world work in Step 2. Consequence: PR-D's `world(published)` public
  surface would ship with a working search box returning default-world hits,
  i.e. drafts, and the row gate cannot catch it (guard rule 1 makes the row
  gate world-independent). Proposed fix: search REFUSES a non-default world
  (`search.ErrScope`) — the Q8 stance — so the wiring cannot be built wrong.
- **RR-CUUZ9Z — the single-entity `Get` path is never world-resolved.** The
  plan is entirely list-shaped; `GetEntity` is contractually the default
  state (`store.go:236-244`) and is what `PolicyReader.Get` and all five
  tracer sites call. A `visibility ∘ world(published)` reader would return
  the DRAFT for `GET /api/v1/entities/{id}` while its list path returns the
  published face.

Findings folded into the plan above (RR-MNOBJK → Test Plan/Risks,
RR-E1C216 → AC5, RR-LLLBQY + RR-EHER1V + RR-KNDLGR → Files/Approach,
RR-CGRV0X → Q4 risk row, RR-CZN30X → Technical Approach, RR-NJSCP5 → PR-C
scope). See the amendments marked **[design review]**.
