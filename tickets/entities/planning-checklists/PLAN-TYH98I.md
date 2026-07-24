---
id: PLAN-TYH98I
type: planning-checklist
title: 'Planning: rela-docs phase 2 (Tier A): markdown+Lua-island doc language + schema/graph resolvers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Phase 2 (Tier A) of FEAT-G4VO53, reframed as a **doc language** (RES-EK7LSA
addendum): markdown authored by a human, with mechanical fragments *pulled* from
the schema/seeded-graph via Lua islands. `rela docs build <manual.md>` runs a
preprocessor over the islands and emits resolved Markdown (pandoc-able to PDF).

**Runtime model (confirmed with user):** ONE graph.
- **Schema** (`*metamodel.Metamodel` + `*acl.Policy`) is loaded from the **real
  project** (`--project` dir).
- **Entities/relations** come **only** from what the manual seeds via
  `create`/`link` into a **fresh ephemeral `memstore`**, discarded post-build.
- No real-project entity reads. So `entity{}`/`count{}`/`graph{id}` read the
  seeded fixture — author-controlled, self-contained. No live-data concern, no
  `--allow-live-data` gate, no two-graph split.

**Seed posture (confirmed with user): RAW STORE, no validation.** The `create`/
`link` seed bindings write **directly to `memstore`'s `store.Store` surface**
(`CreateEntity(ctx,e) error` / `CreateRelation`) — NOT through
`entitymanager.Manager`. No ACL, no automations, no state-machine `Initial`/
legal-value gate. Honest "fixture" semantics: the author places exactly the
entities they want to illustrate (`create("risico",{status="done"})` just
works). This resolves design-review C1 — the doc runtime needs NO
`entitymanager` and none of its 6 collaborators; the seed bindings are new thin
bindings over `store.Store`, distinct from the existing `Mutator`-based
`rela.create_entity`.

**Determinism is NOT a goal (confirmed with user).** Islands MAY call `ai.*`/
`http.*`/`today` (e.g. AI-generated prose) — a feature, not a bug. So the doc
runtime routes through the normal runtime wiring; no need to neutralize
time/net/AI bindings. (Resolves design-review S5.) The build is "stable given
stable inputs," not byte-stable across days/network.

IN scope:
- **Island preprocessor**: markdown by default; a fenced ` ```rela ` **statement
  island** (doc-API calls append to an output buffer at that position, PHP-echo
  model; loops/conditionals are plain Lua) and an inline `` `rela <expr>` ``
  **echo island** (substitutes the string value mid-sentence).
- **Doc runtime**: a Lua runtime whose `Meta` = real project metamodel and whose
  `Store`/`Tracer` = a fresh **memstore** + `tracer.New(memstore)`, plus a
  captured output buffer for statement islands, and a bounded `WithTimeout`
  (resource-exhaustion guard, see below). Seed = **new thin `create`/`link`/
  `update` bindings that call `memstore`'s `store.Store` methods directly** (raw
  store, no entitymanager — see Seed posture). NOT the existing `Mutator`-based
  `rela.create_entity`.
- **Binding namespace (settles design-review S1 / open-Q1):** resolvers +
  emit helpers + seed live under a top-level **`doc.*`** table (e.g.
  `doc.typeref{}`, `doc.h2()`, `doc.create()`), registered by `internal/docs`
  as its own module (the in-tree `registerCryptoModule` pattern, `runtime.go:677`)
  — NOT added to the `rela.*` table and NOT as `(*Runtime)` methods (keeps the
  plimsoll god-object count flat). A thin preamble may alias the common ones
  (`typeref = doc.typeref`, `h2 = doc.h2`, `md = doc.md`) into the island's
  global scope for ergonomics, decided in impl.
- **Tier-A resolver bindings** (in the `doc.*` module):
  `typeref{type, fields}`, `values{type, field}`, `relations{type}`,
  `graph{from, depth, exclude/only, direction}` (mermaid subgraph),
  `lifecycle{type, field}` (mermaid stateDiagram or flat-list fallback),
  `entity{id, fields}`, `count{type}`, `roles_matrix{type}`, `description()`,
  plus emit helpers `h1/h2/h3/md(...)` and seed `create/link/update`.
- **`internal/mermaid`** new package: extract `mermaidStateDiagram`+`mermaidLabel`
  from `internal/dataentry/handlers.go` + a `CustomType`→diagram mapping; both
  `dataentry` and `docs` import it (dataentry refactored to use it).
- **`rela docs build`** command + a `--strict` knob (empty-resolve policy).
- An **example manual** under `prototypes/` (ISMS-style corpus) + user docs.

OUT of scope:
- **Tier B / `screenshot{}`** — the Playwright harness (phase 3).
- Any browser dependency, data-entry SPA rendering, PNG capture.
- Changing `tracer` (we post-filter its output; see Approach).
- Real-project entity reads (seeded memstore only).
- A push-generator that emits a whole `.md` with no source manual. (A future
  `rela docs scaffold` that generates a manual-of-islands is a follow-up, not
  this ticket.)

**Acceptance Criteria:**

1. **Statement island** — a ` ```rela ` block calling `h2("X"); md("y")`
   produces `## X\n\ny` at that position in the output; surrounding markdown is
   passed through verbatim. (Test: preprocess a fixture string, assert output.)
2. **Echo island** — inline `` `rela count{type="risico"}` `` substitutes the
   count into the sentence; a code span WITHOUT the `rela` marker is left
   untouched. **Coercion (settles S2):** the preprocessor wraps the island body
   as `return <expr>`; the returned Go value → string is: `string`→itself,
   `int64`/`float64`/`number`→`fmt.Sprint`, `nil`→empty (or error under
   `--strict`), `bool`→error, `table`→error (author placed a block resolver in
   an inline span — fail loud). An empty/absent expression (`` `rela ` ``) →
   error naming the line. (Test: each coercion case + empty.)
3. **`typeref{type="risico", fields="required"}`** emits a markdown table of the
   required properties (name, type, required) read from the metamodel, in
   `PropertyOrder`. (Test: against a fixture metamodel.)
4. **`values{type="risico", field="behandeling"}`** emits the enum values +
   default, and per-value meaning when `CustomType.Descriptions` is present.
   (Test: with and without descriptions.)
5. **`lifecycle{type, field}`** emits a mermaid `stateDiagram-v2` when the
   custom type has `Transitions`; falls back to a flat value list when it does
   not. Diagram is injection-safe (synthetic state ids). (Test: both; + a value
   containing mermaid-breaking chars asserts no injection.)
6. **`graph{}`** — TWO grains (settles S3):
   - `from="<type>"` builds the **schema** neighbourhood from `Meta` relations
     (which types connect to which); depth = hops in the type graph.
   - `from="<id>"` traverses the **seeded memstore** via tracer; depth = hops in
     the instance graph.
   Emits mermaid `graph LR`. **`direction` filters by edge orientation
   (`.Incoming`), which is SEPARATE from `exclude`/`only` (which filter by
   relation TYPE `.Relation`)** — the two are distinct fields. `direction="in"`
   uses `TraceTo`; `direction="out"`/`"both"` uses `TraceFrom` then post-filters
   `.Incoming`. `exclude`+`only` both set → **error** (contradictory, settles
   M2). `depth` omitted → **default 2**; hard-capped at **5** (settles C2).
   Flatten dedupes nodes by id and edges by (from,rel,to) triple.
   (Test: seed a **diamond-shaped** graph — asserts the exact deduped edge set,
   catching the tracer `visited` lossy-leaf bug; + depth honored; + exclude/only
   prune; + a type-grain case.)
7. **`roles_matrix{type="risico"}`** emits a role×verb table (create/read/update/
   delete ✓) from the acl policy via `grantsVerb`. (Test: fixture policy.)
8. **Seed** — a statement island calling `create("risico", {...})` then
   `entity{id=...}` renders the seeded entity's fields; the write went to the
   memstore, and the real project on disk is untouched. (Test: assert store
   isolation.)
9. **Fail-loud** — an island referencing a nonexistent type/field, or a Lua
   error, aborts the build with an error naming the **manual's source line**
   (not the island-internal line — a multi-line island at manual line 40 that
   errors on its 2nd line reports `manual.md:41`, settles M1) and the offending
   island. Under `--strict`, an empty resolve (e.g. `description()` with no
   top-level description) also errors; without it, warns + emits empty.
   (Test: each failure mode + the line-offset case.)
10. **`rela docs build manual.md`** end-to-end: an ISMS-style manual (in-tree
    fixture, no external corpus — settles N3) with prose + islands produces a
    complete resolved `.md`; exit 0. Output is **stable given stable inputs**
    (NOT byte-stable across days/network — determinism is not a goal; islands
    may use time/net/ai). Golden test uses a manual that touches no
    time/net/ai bindings so it stays comparable. (Integration test.)
11. **`internal/mermaid`** extraction — `dataentry` help modal still renders the
    same diagram (no regression). The shared renderer takes a **neutral
    primitive input** (`initial string; transitions []Transition{From,To,Label}`),
    NOT `CustomType` and NOT `EnumHelp` — both callers map into it, reproducing
    the SAME label fallback (`Label`→`Labels[To]`→`To`) so the two paths can't
    drift (settles S4). New `.go-arch-lint.yml` component + `mayDependOn` edits
    are mandatory. (Test: shared renderer unit tests + dataentry golden diagram.)
12. **Resource bounds (settles C2)** — the doc runtime is built with a bounded
    `WithTimeout` (default 30s inherited, or explicit); an island with an
    infinite loop aborts at the deadline, not hangs. (Test: a `while true` island
    times out with a clear error.)

## Research

- [x] ~~For larger features: run `/research`~~ (RES-EK7LSA covers this arc; the
  2026-07-21 addendum captures the full doc-language design validated against
  the ISMS corpus. No new research doc needed.)
- [x] Searched for existing libraries — reuse in-tree gopher-lua runtime; no new dep.
- [x] Checked codebase for similar patterns or reusable code (two Explore sweeps).
- [x] Looked for reference implementations — openvwr (studied), document-mode path.
- [x] Reviewed relevant rela concepts for prior art.

**Research Doc:** RES-EK7LSA (see the ADDENDUM 2026-07-21 for the doc-language design).

**Existing Solutions (grounded, file:line):**

- **Lua runtime** — `internal/lua`, gopher-lua (`go.mod:26`). Sandboxed
  (`SkipOpenLibs`, no io/os, `runtime.go:272/326`). `lua.NewReader(ReadDeps,...)`
  (`runtime.go:249`) / `lua.NewWriter(WriteDeps,...)` (`:258`). Bindings are
  `func(*lua.LState) int`; table-in via `luaTableToGoMap`, table/value-out via
  `EntityToTable` (`:1080`) / `GoToLuaValue` (`:1207`). Read bindings registered
  in `registerReadBindings` (`:682`) — **the extension point for new resolvers.**
- **Document mode = the closest analogue.** `Engine.ExecuteDocument(path, deps,
  stdout, docID, entryID, timeout)` (`internal/script/executor.go:98`) already
  runs a Lua runtime and captures its **stdout as rendered markdown**
  (`WithDocumentMode`, `runtime.go:206`). Our statement-island emit buffer is the
  same idea; we can model the doc runtime on this path.
- **ReadDeps/WriteDeps** — `internal/lua/deps.go:21/51`. `WriteDeps.Store` +
  `WriteDeps.EntityManager` (a 5-method `Mutator`, `deps.go:40`) is exactly the
  seed seam: point `Store` at `memstore.New()` and supply a memstore-backed
  `Mutator`. `Meta *metamodel.Metamodel` stays the real project's.
- **Write bindings** (the seed API) — `rela.create_entity` (`runtime.go:1409`),
  `rela.create_relation` (`:1641`), `rela.update_entity` (`:1451`), registered in
  `registerWriteBindings` (`:729`). No new API.
- **Wire through `script.NewWriterRuntime`** (`internal/script/runtime.go:20`) so
  AI/secrets load consistently (per CLAUDE.md, don't call `ai.LoadProvider`).
- **Schema walk template** — `internal/cli/schema.go`: `writeEntityProperties`
  (:301), `writePropertyDetail` (:320, incl. enum fallback to
  `meta.Types[type].Values` at :338), `writeEntityRelations` (:346),
  `getSortedEntityNames` (:819). Clone the walk shape for the resolvers.
- **Metamodel types (all doc-fields CONFIRMED present)** —
  `Metamodel.Description` (`types.go:23`), `EntityDef.Description` (:213) +
  `PropertyOrder` (:222, stable order), `PropertyDef.{Description,Labels,Values,
  Required,List}` (:260-314), `RelationDef.{Description,From,To,Inverse}` (:429),
  `CustomType.{Values,Labels,Descriptions,Default,Description,Transitions,Initial}`
  (:136-164), `TransitionDef.{From,To,Label,Help,Guard,When}` (:170-198). **No
  metamodel changes needed** — phase 1 already added everything.
- **tracer** — `internal/tracer/tracer.go`. `TraceFrom(ctx,id,maxDepth)` (:48,
  bidirectional), `TraceTo` (:51, incoming), result `TraceResult{...,Relation,
  Incoming,Children}` (:24). `tracer.New(reader)` (:70); a `memstore` satisfies
  the reader. **NO edge-type filter** — decision below.
- **mermaid (Go-side, unexported)** — `mermaidStateDiagram(EnumHelp)`
  (`internal/dataentry/handlers.go:297`, synthetic-id injection-safe),
  `mermaidLabel` (:348). Keyed on `EnumHelp` (dataentry-local), NOT `CustomType`.
  `internal/htmlutil/mermaid.go` has fence-rewrite helpers (different concern).
- **acl** — `internal/acl/policy.go`: `Policy.{Description,Roles,...}` (:87),
  `RoleDef.{Description,Create,Read,Update,Delete,Permissions}` (:172),
  **`grantsVerb(role,op,target)` (:196) = the matrix cell function.**
  `LoadPolicy(path)` (:340, `errors.Is(err,os.ErrNotExist)` for "no policy").
  acl deliberately does not import metamodel (arch-lint); role names + entity
  types are strings — the matrix is a pure string cross-product.
- **memstore** — `internal/store/memstore`, **no build tag, directly importable.**
  `memstore.New(opts...) *MemStore` (`memstore.go:95`), satisfies `store.Store`
  and tracer's reader. Metamodel is a **separate collaborator**, not loaded into
  the store.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Design decisions (locked with user):**

- **`graph{}` edge filtering → post-filter in the doc layer.** Call
  `tracer.TraceFrom(ctx,id,depth)` unchanged, then prune `TraceResult.Children`
  by `.Relation` against `exclude`/`only` in the doc package. Tracer stays a
  clean pure-reader; filtering logic lives with the feature; smallest blast
  radius. (Rejected: adding a filter param to the shared tracer — bigger review
  surface, storetest implications, not needed yet.)
- **Mermaid renderer → new `internal/mermaid` package.** Move
  `mermaidStateDiagram`/`mermaidLabel` there, add a `StateDiagram(ct CustomType,
  ...)` entry that builds the diagram directly from a `CustomType` (mapping
  `Initial`/`Transitions`/`Labels`). Refactor `dataentry` to call it (its
  `EnumHelp`→diagram path delegates). One injection-safe renderer, shared.
- **One graph, seeded memstore, RAW-store seed (resolves C1).** The seed
  bindings write directly to `memstore` via its `store.Store` methods
  (`CreateEntity(ctx,e) error`, `CreateRelation`) — memstore satisfies
  `store.Store`, so **no `entitymanager` and none of its 6 collaborators are
  needed.** Schema resolvers read the real `Meta`; instance resolvers + seed use
  the memstore + `tracer.New(memstore)`. acl loaded separately via
  `acl.LoadPolicy(realRoot/acl.yaml)`. The doc runtime is thus a **reader-shaped
  runtime plus a small raw-store write surface** — it does NOT need the full
  `WriteDeps`/`Mutator` machinery.
- **Determinism dropped (resolves S5).** Route through the normal runtime; `ai`/
  `http`/`today` stay available to islands (a feature). Golden test avoids them.
- **`--output` file semantics (resolves S6).** The manual path and `--output`
  are **operator-supplied, arbitrary, typically absolute** paths — same trust as
  `pandoc in.md -o out`. Use plain `os.ReadFile`/`os.WriteFile`; NO project-root
  traversal check (`loadScript`'s `filepath.IsLocal`+`.lua` guard does not apply
  and would wrongly reject an absolute `.md`). `--output` overwrites; refuse if
  it names a directory; create parent dirs. Default output: stdout if `--output`
  omitted.

**Technical approach (hard-first order per user "start with the hard stuff first"):**

1. **`internal/mermaid` extraction** (do first — unblocks `lifecycle`/`graph`
   and de-risks the regression). Define a **neutral primitive input** —
   `type Transition struct{ From, To, Label string }`, `StateDiagram(initial
   string, ts []Transition) string` + `Graph(nodes []Node, edges []Edge) string`
   (LR). Move `mermaidLabel` + synthetic-id logic. Refactor `dataentry/
   handlers.go` so its `EnumHelp`→diagram path maps into `StateDiagram` (NOT the
   reverse — mermaid must not import dataentry). Reproduce the label fallback
   (`Label`→`Labels[To]`→`To`) once, in the `CustomType`→`[]Transition` mapper
   the doc side uses, matching dataentry's `Move` fallback so they can't drift
   (S4). **Add `mermaid` to `.go-arch-lint.yml`** + `dataentry`/`docs`
   `mayDependOn`. Keep dataentry tests green + golden-compare the diagram.

2. **The doc runtime + island preprocessor** (the architectural core, riskiest):
   - New package `internal/docs` (CLI wiring in `internal/cli`).
   - **Preprocessor**: a scanner that splits a markdown file into
     literal-passthrough spans, ` ```rela ` fenced statement islands, and inline
     `` `rela <expr>` `` echo spans. Track each island's **manual source line**
     (for M1 fail-loud offset). (Non-`rela` fences/spans pass through untouched.)
   - **Runtime**: build a runtime with real `Meta` + a fresh `memstore` +
     `tracer.New(memstore)` + a bounded `WithTimeout`. Register the **`doc.*`
     module** (resolvers + emit + raw-store seed bindings) via a `registerDocModule(r)`
     helper mirroring `registerCryptoModule` (`runtime.go:677`) — closures over a
     `docDeps` struct, NOT `(*Runtime)` methods. Statement island: run the chunk,
     `doc.h*/doc.md` append to an output `*strings.Builder`; splice at position.
     Echo island: **wrap body as `return <expr>`**, run via `RunActionString`
     (`runtime.go:492`), coerce the returned `any`→string per AC2.
   - **Fail-loud**: wrap every island run; map the Lua error frame back to the
     **manual line** (island start line + intra-island offset − 1, M1); abort
     with `manual.md:line` + island text. `--strict` promotes empty-resolve to error.
   - Prove statement vs echo end-to-end with just `doc.md()`/`doc.h2()`/`count{}`
     before writing the rest of the resolvers.

   Decision on runtime construction: since the seed is **raw-store** (no
   entitymanager) and determinism is not required, the doc runtime can be built
   with `lua.NewReader(ReadDeps{Store: memstore, Meta: realMeta, Tracer: ...})`
   for the read surface, plus the `doc.*` module supplying raw-store
   `create`/`link`. Confirm during impl whether `NewReader` + a doc-write module
   is cleaner than `NewWriter` (the latter drags in the `Mutator` surface we do
   NOT want). Lean: `NewReader` + doc module.

3. **`graph{}`** — TWO code paths behind one name:
   - **id grain**: `direction="in"`→`tracer.TraceTo`; `"out"`/`"both"`→
     `tracer.TraceFrom` then post-filter children by `.Incoming` (direction) AND
     `.Relation` (exclude/only — SEPARATE fields, S3). Flatten with node-dedupe
     (by id) + edge-dedupe (by from,rel,to) to defeat the `visited` lossy-leaf
     (S3). `depth` default 2, cap 5 (C2).
   - **type grain**: walk `Meta.Relations` for edges touching the type, N hops;
     no tracer. Render via `internal/mermaid.Graph`.
   `exclude`+`only` both set → error (M2). All node/edge labels through the
   injection-safe path.

4. **`lifecycle{}`** — `internal/mermaid.StateDiagram(CustomType)` or flat-list
   fallback when no `Transitions`.

5. **`roles_matrix{}`** — rows/cols from **BOTH** sources (M3): entity-type list
   from `Meta.Types`/entity defs, verb grants from `Policy.Roles` via
   `grantsVerb` (+ read). Metamodel entity types drive the rows (a role granting
   a verb on an undeclared type is ignored with a warning). Markdown table.
   acl.yaml absent → `LoadPolicy` returns `os.ErrNotExist` → emit a clear
   "no policy defined" note, not a crash.

6. **Mechanical resolvers** — `typeref` (mirror `writeEntityProperties`),
   `values`, `relations` (mirror `writeEntityRelations`), `entity`, `count`,
   `description`.

7. **Seed bindings (`create`/`link`/`update`) — raw store, NOT aliases of the
   existing write bindings.** New `doc.*` bindings that call `memstore`'s
   `store.Store` methods directly (`CreateEntity(ctx,e) error` from
   `memstore/tx.go`, `CreateRelation`). No entitymanager, no validation (per
   confirmed seed posture). `link(from, type, to)` = `CreateRelation`. This is
   real (small) work, NOT the "naming/ergonomics" the earlier draft claimed —
   the design-review correctly flagged that mislabel; the raw-store decision is
   what makes it small.

8. **CLI + example + docs** — `rela docs build <manual>` in `internal/cli`
   (register in `requiresProject`), `--strict` flag, `--output` path; an example
   manual under `prototypes/data-entry/` (reuse the phase-1 corpus); a
   `docs/rela-docs.md` guide (via the source-guide pipeline) + CLAUDE.md pointer.

**Files to modify / add:**
- ADD `internal/mermaid/{statediagram.go,graph.go,label.go,*_test.go}` (neutral
  primitive input types).
- EDIT `internal/dataentry/handlers.go` (map `EnumHelp`→`mermaid.StateDiagram`).
- ADD `internal/docs/{preprocess.go,runtime.go,resolvers_*.go,seed.go,*_test.go}`.
- The `doc.*` module is registered from `internal/docs` (closures over a deps
  struct), invoked after runtime construction via `LState()` or a small exported
  hook — NO new methods on `*Runtime` (plimsoll). Confirm the exact seam in impl;
  avoid editing `internal/lua/runtime.go`'s core registration if possible.
- EDIT `.go-arch-lint.yml` — add `mermaid` + `docs` components and `mayDependOn`
  (mandatory, not optional — `just arch-lint` gates the PR).
- ADD `internal/cli/docs.go` (the `rela docs build` command) + wire in the CLI
  tree; register in `requiresProject`.
- ADD an in-tree example manual (e.g. `prototypes/data-entry/manual/*.md`) +
  `docs-project/entities/guides/GUIDE-rela-docs.md` (source guide) →
  `docs/rela-docs.md` via `just docs` (confirmed with user: source-guide
  pipeline, matching metamodel.md/acl-overview.md).

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- **The manual `.md` + its Lua islands** — operator-authored, same trust level as
  a scheduler script or `metamodel.yaml`. Islands run in the **already-sandboxed**
  gopher-lua state (no io/os/debug; `loadfile`/`load` nilled — `runtime.go:326`).
  Write bindings target a **throwaway memstore**, never the real store, so a
  malicious/buggy seed cannot corrupt project data.
- **Resolver args** (type/field/id strings) — validated against the metamodel /
  seeded store; unknown refs **fail loud** (allowlist by construction: only
  declared types/fields resolve).
- **Mermaid injection** — the #1 concrete risk, carried from phase 1a.5. All
  enum values / labels / node names routed through `internal/mermaid`'s
  synthetic-id + `mermaidLabel` newline-flattening. **AC5 asserts a
  mermaid-breaking value does not inject.** graph node labels get the same
  treatment.
- **Path handling (corrected per S6)** — the manual and `--output` are
  operator-supplied arbitrary paths (a doc, not a project script), same trust
  boundary as `pandoc in.md -o out`. Plain `os.ReadFile`/`os.WriteFile`; **no**
  project-root/`filepath.IsLocal` check (that guard is for untrusted
  project-relative script loading and would wrongly reject an absolute `.md`).
  No shell interpolation. `--output` refuses a directory target.
- **Resource exhaustion (C2)** — TWO vectors, both bounded:
  (a) **infinite loop in an island** — `CallStackSize:1024` stops recursion only;
  a flat `while true` is stopped solely by the context deadline
  (`applyTimeout`→`L.SetContext`, `runtime.go:577`; gopher-lua honors ctx-cancel
  at VM points). **The doc runtime MUST be built with a bounded `WithTimeout`**
  (AC12) — do not rely on inheriting `DefaultTimeout` implicitly; set it.
  (b) **unbounded tracer** — `graph{}` omitted `depth` → `TraceFrom(id,0)` →
  `maxDepth<=0` is **unbounded** (`tracer.go:96`). Mitigation: `graph{}` defaults
  `depth` to **2** and hard-caps at **5**; never passes `<=0` to the tracer.

**Security-Sensitive Operations:**
- Lua execution — contained by the existing sandbox (no io/os/debug; `loadfile`/
  `dofile`/`load`/`loadstring`/`setmetatable`/`raw*` nilled at
  `runtime.go:347-350`; `coroutine` present but bounded by the timeout). This is
  the *sanctioned* place the "no user Lua on the read path" rule does not bite
  (offline operator build; seed writes land in a throwaway memstore). Read-path
  hot-loop concerns do not apply — one-shot build.
- Seed writes — raw `store.Store` on a throwaway memstore; cannot touch the real
  project. The real project dir is read-only for the whole build (metamodel +
  acl loaded, entities never read).
- File write (`--output`) — single operator-chosen output file.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios (map to ACs):**
- Preprocessor unit tests (AC1/AC2): table-driven over source strings →
  expected spliced output; incl. non-`rela` fence/span passthrough, source-line
  tracking, statement-vs-echo.
- Resolver unit tests (AC3-7): each resolver against a small **fixture
  metamodel** + (for instance resolvers) a **seeded memstore**. Build fixtures
  with `memstore.New()` + a hand-built `*metamodel.Metamodel` (`InitAliases()`)
  + `acl.LoadPolicyBytes` for the matrix — **no `appbuildtest`/`entitymanager`
  needed** now the seed is raw-store (this is a direct benefit of the C1
  resolution; the earlier draft wrongly assumed a full manager). Seed test data
  via `memstore.CreateEntity`/`CreateRelation` directly.
- Seed isolation (AC8): seed via `create`, read via `entity`, assert the real
  project dir is untouched (memstore is a separate object; the build never opens
  the real entities dir for writes). Raw-store seed means `create` always
  succeeds regardless of state-machine `Initial` — assert `{status="done"}`
  seeds without a transition error (the fixture-semantics guarantee).
- Fail-loud (AC9): bad type, bad field, Lua syntax error, `--strict` empty
  resolve — each asserts an error mentioning file:line.
- Integration (AC10): a committed example manual → golden resolved `.md`
  (deterministic; assert byte-stable across runs).
- Mermaid regression (AC11): `internal/mermaid` unit tests + the existing
  dataentry help-modal test stays green.

**Edge Cases:**
- Empty manual / manual with no islands → passthrough unchanged.
- Island resolving to empty (`description()` w/ no top-level desc) → `--strict`
  errors, else empty + warning.
- Enum with no `Descriptions` → `values` emits values only (AC4 negative).
- Custom type with no `Transitions` → `lifecycle` flat-list fallback (AC5).
- `graph` depth 0 / unbounded (tracer treats `<=0` as unbounded — clamp/validate
  a sane max to avoid a runaway on a large seed).
- `exclude` + `only` both set → `only` wins (documented) or error (decide;
  lean: error, contradictory).
- Unicode / mermaid-breaking chars in values, labels, ids (injection).
- Duplicate seed id / invalid seed props → entitymanager warning surfaced.

**Negative Tests:**
- Unknown `type`/`field`/`id` → fail-loud with file:line.
- Malformed island (unterminated expr) → parse error with line.
- `rela docs build` with no `--project` → clear error (command requires project).
- acl.yaml absent → `roles_matrix` degrades (no policy) with a clear message,
  not a crash.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **Preprocessor/runtime plumbing is the novel part** (everything else mirrors
  existing walks). Mitigation: model on the existing document-mode stdout-capture
  path; prove statement/echo end-to-end (step 2) before resolvers.
- **`internal/mermaid` extraction could regress the help modal.** Mitigation: do
  it first, keep dataentry tests green, golden-compare the emitted diagram.
- **Registering doc bindings without editing core `internal/lua`.** Prefer an
  `Option` that registers extra bindings from `internal/docs`; fall back to a
  small core hook only if needed. Keeps the god-object/plimsoll surface of the
  runtime from growing.
- **arch-lint boundaries** — `internal/docs` may import
  `metamodel`/`acl`/`tracer`/`store`/`memstore`/`lua`/`script`; must NOT pull
  browser/dataentry-SPA deps. Run `just arch-lint` early.
- **Effort: l** (new package + runtime + ~9 resolvers + CLI + mermaid extraction
  + example + docs). Could split into l+m if the resolver set is staged, but the
  user asked for "all of it" — keep as one l ticket, staged internally.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/cli-reference.md` — the new `rela docs build` command + flags.
- [x] A new `docs/rela-docs.md` guide — the doc language: island syntax, the
  resolver reference, the seed/memstore model, fail-loud/`--strict`. Authored via
  the source-guide pipeline if one applies, else a plain doc.
- [x] CLAUDE.md — a short pointer to the doc-language + `internal/docs` +
  `internal/mermaid` (new subsystems).
- [x] The committed **example manual** doubles as living documentation.
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel changes — phase-1 fields already exist).

## Design Review

- [x] Run `/design-review` before starting implementation (cranky reviewer, verified against real code)
- [x] All critical/significant findings addressed in plan

**Design Review Findings** (all addressed in-plan; no review-response entities
needed since fixes landed directly in this plan before implementation):

- **C1 (critical) — seed path wrong: `memstore` ≠ `Mutator`.** Correct: only
  `entitymanager.Manager` satisfies `Mutator`, and it needs 6 collaborators +
  runs the real state-machine/ACL/automations. **RESOLVED** by the confirmed
  **raw-store seed posture**: seed bindings call `memstore`'s `store.Store`
  methods directly, no entitymanager at all. Simplifies the runtime.
- **C2 (critical) — resource bounds unwired.** RESOLVED: explicit `WithTimeout`
  on the doc runtime (AC12) + `graph{}` depth default 2 / cap 5 (never passes
  `<=0` to the unbounded tracer). See Security.
- **S1 (significant) — binding namespace / registration seam.** RESOLVED: a
  top-level **`doc.*`** module registered `registerCryptoModule`-style (closures,
  not `*Runtime` methods → plimsoll flat). Namespace locked now.
- **S2 (significant) — echo coercion undefined.** RESOLVED: AC2 pins the
  `return <expr>` wrapping + the `any`→string coercion table.
- **S3 (significant) — `direction` vs relation-filter conflated; tracer is
  bidirectional + lossy `visited`.** RESOLVED: AC6 separates `direction`
  (`.Incoming`, chooses `TraceTo`/`TraceFrom`) from `exclude`/`only`
  (`.Relation`); node/edge dedupe on flatten; diamond-graph fixture asserts the
  exact edge set; type-grain gets its own path/AC.
- **S4 (significant) — mermaid input type + drift + arch-lint.** RESOLVED: AC11
  mandates a **neutral primitive input** (not `CustomType`/`EnumHelp`), one
  shared label-fallback, and the mandatory `.go-arch-lint.yml` edits.
- **S5 (significant) — determinism vs always-on ai/http/time.** RESOLVED:
  determinism is **not a goal** (confirmed); AC10 relaxed to "stable given
  stable inputs"; islands may use ai/http/time.
- **S6 (significant) — cited traversal helper doesn't apply.** RESOLVED:
  Security section corrected — plain `os.ReadFile`/`WriteFile`, operator trust
  boundary, no `filepath.IsLocal` guard.
- **M1 — fail-loud must report the MANUAL line, not the island-internal line.**
  RESOLVED: AC9 pins the offset arithmetic + a multi-line-island test.
- **M2 — `exclude`+`only` both set → error** (locked, AC6).
- **M3 — `roles_matrix` reads BOTH `Meta` and `Policy`** (rows from metamodel
  types) — documented in step 5.
- **M4 — time bindings vs golden test** — golden manual avoids time/net/ai
  bindings (AC10).
- **N1/N2/N3 — cite fixes** (`runtime.go:347-350`, coroutine note, in-tree
  fixture only) folded into Security + AC10.
