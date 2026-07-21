---
id: RES-EK7LSA
type: research
title: Generating end-user + operator docs from a rela deployment (metamodel + acl.yaml -> Markdown)
summary: 'Recommend: add 4 minimal doc-fields (metamodel/CustomType-value/TransitionDef/RoleDef descriptions) first, then a `rela docs --output-dir` generator emitting one Diataxis-informed Markdown (reference tables + prose + mermaid state diagrams + role x entity capability matrix).'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Problem

A rela deployment (ticket tracker, ISMS, project tracker, ...) is fully
described by its `metamodel.yaml` (entity types, properties, relations, custom
types / state machines, automations, validations) + `acl.yaml` (roles,
permissions, role-relations, assignments). Today the end-user and
operator/support documentation for such a deployment is hand-maintained and
drifts from the schema.

Goal: `rela docs --output-dir ./site` generates **one Markdown file** (PDF-
convertible; mermaid fences for diagrams) documenting a specific deployment for:
- **End users** — what the things are, the fields, how they connect, the
lifecycle (state machines), what happens automatically.
- **Operators / support staff** — the role model: who can do what, who can
perform which guarded transitions, what rules must hold.

Decided up front: one big Markdown for v1; new top-level `rela docs` command;
**design the doc-oriented schema fields first, then build the generator.**

## Context

### External best-practice survey

**Diátaxis** (the standard doc framework — diataxis.fr) splits docs into four
modes on two axes (acquisition↔application, action↔cognition):

| Mode | Serves | Can a schema generate it? |
| --- | --- | --- |
| **Reference** | practitioner at work — facts | **YES — the natural fit.** Schema-derived docs excel here: accurate, complete, structural. |
| **Explanation** | learner — the "why" | **NO — needs human prose.** Meaning/rationale can't be derived from structure. |
| **Tutorial** | beginner — guided lesson | **NO — needs human authoring.** Pedagogical sequencing. |
| **How-to** | competent user — a task | **PARTIAL.** Workflows *can* be inferred from transitions + automations. |

**This is the load-bearing insight for the phase split:** a generator produces
excellent *reference*, and can infer *how-to* from lifecycle/automation data,
but the *explanation* layer (what a status MEANS, why a role EXISTS, when to
make a move) is exactly what a schema lacks — so the metamodel/ACL need new
prose fields to carry it. Generating from today's schema alone yields a dry
reference; the doc- field additions are what upgrade it toward genuinely useful
end-user docs. (This directly justifies "additions first, generator second.")

**Roles/permissions presentation** (GitLab docs, access-control-matrix best
practice): a narrative **role table upfront** (each role + a one-line "what it's
for"), then permissions **grouped by feature area** as tables with **roles as
columns, actions as rows, ✓ = granted**, plus footnotes for conditional nuance.
Model roles/groups as subjects (not individuals). State the rationale for each
role — "essential for audits and training new administrators." For rela the
"feature areas" map cleanly to **entity types** (per-type: who can
create/read/update/delete, who holds each transition guard).

**Lifecycle/workflow** (Jira workflow docs): a status is a single state; a
transition is a one-way verb-named link ("transition names should tell the user
what to do — think verbs"); the available transitions are shown on the item
view. rela already matches this exactly (TransitionDef.Label = the verb).
Mermaid `stateDiagram-v2` renders it: `[*] --> <Initial>` for entry + `From -->
To : <Label>` per edge, guard/precondition annotated in the edge label.

Sources: [Diátaxis](https://diataxis.fr/start-here/), [GitLab
permissions](https://docs.gitlab.com/user/permissions/), [Access-control matrix
best practice](https://frontegg.com/blog/access-control-matrix), [Jira
workflows](https://support.atlassian.com/jira-cloud-administration/docs/work-with-issue-workflows/),
[Mermaid state diagrams](https://mermaid.js.org/syntax/stateDiagram.html).

### Internal survey — what the schema already carries

Everything the generator reads is available from `*metamodel.Metamodel` +
`*acl.Policy`, read-only (confirmed via Explore + code read):

- **Entity types**: `Metamodel.EntityTypes()`, `EntityDef{Label, LabelPlural,
Description, PropertyOrder, Properties}` — `GetPropertyOrder()` gives properties
in definition order.
- **Properties**: `PropertyDef{Type, Required, Values, Labels, Default,
Description, List, ...}`.
- **Relations**: `RelationDef{Label, Description, From, To, Inverse (GetLabel),
Min/MaxOutgoing/Incoming, Symmetric}` — outgoing/incoming per type derivable
(mirror `schema.go writeEntityRelations`).
- **State machines**: `CustomType{Values, Labels, Default, Description, Initial,
Transitions[]}`; `TransitionDef{From, To, Label, Guard, When}`. Fully
declarative — **the generator needs only `metamodel`, NOT the compiled
`statemachine.Set`** (that adds only runtime/principal evaluation, wrong for
docs).
- **Automations**: `Metamodel.Automations[] {On{Entity,Property,Becomes,From,...},
Do[]{Set/Value, CreateRelation, CreateEntity, Lua}}` —
`On.Property`+`On.Becomes` is precisely "when status becomes X, this happens",
narratable for how-to.
- **Validations**: `Metamodel.Validations[] {Name, Description, When, Then,
Severity}` — "rules that must hold".
- **ACL**: `Policy{Roles map[string]RoleDef, Assignments map[string]string,
RoleRelations}`; `RoleDef{Create/Update/Delete/Read []string, Permissions
[]string, ...affordance grants}`. Roles × entity-types × verbs + which roles
hold which transition-guard permissions = the operator matrix.

**Reuse seam**: mirror `internal/cli/schema.go` (already walks the metamodel to
text/JSON/graphviz — `writeEntityProperties`, `writePropertyDetail`,
`writeEntityRelations`, sorted-name helpers). New top-level `rela docs` command
taking `*readServices` (gives `svc.Meta`) + `--output-dir`; register in
`requiresProject`. File-writing precedent: `internal/cli/apps.go`. Depend only
on `internal/metamodel` + `internal/acl`.

### The gaps — doc-oriented fields that DON'T exist yet (confirmed by code read)

| Missing field | Where | Carries (the "explanation" layer) |
| --- | --- | --- |
| top-level `description` / `title` | `Metamodel` root | "What is this app / deployment for" |
| per-value descriptions | `CustomType` (has `Labels`, no descriptions) | what each status/enum value *means* |
| `help` / `description` | `TransitionDef` (has `Label`, no help) | why/when to make this move |
| `description` | ACL `RoleDef` (+ optional top-level policy desc) | what each role is for — the audit/operator rationale |

All additive, optional, backward-compatible — same shape as existing
`Description`/`Label` fields. ACL fields are informational only (no enforcement
impact); `analyze_*`/write paths ignore them.

### Constraints

- No new deps beyond `metamodel`/`acl` (arch-lint). `just docs` already exists
(Lua content pipeline) — naming overlap at project level only, not in the CLI;
`rela docs` is fine.
- Mermaid rendering deferred by choosing Markdown output: mermaid fences render
in GitHub/GitLab/most Markdown→PDF pipelines with no bundled JS. (An HTML site
would reopen the mermaid-JS question — out of scope for v1.)
- ACL `When:` predicates and automation Lua are opaque strings — the generator
surfaces them verbatim (or a simplified gloss), not an interpretation.

## Options

### Field additions (Phase 1) — how much to add

**A1. Minimal four fields** (recommended). Add exactly the four gaps above
(metamodel description, CustomType per-value descriptions, TransitionDef help,
RoleDef description). Small, additive, each independently useful. Effort: S.

**A2. Richer doc block per entity.** Add a structured `docs:` sub-block on
EntityDef (intro, usage notes, examples). More expressive but invites scope
creep and a mini-CMS-in-YAML; the overlay-file idea (rejected earlier) was the
alternative to this. Effort: M. Rejected — start with A1, extend if output reads
thin.

### Generator output structure (Phase 2)

**B1. Diátaxis-informed single Markdown** (recommended):
1. **Overview** — top-level description, entity catalog (one line each).
2. **Per entity type** (the bulk): description; **Fields** table (name, type,
required, allowed values + per-value meaning); **Relationships** in prose;
**Lifecycle** = mermaid stateDiagram + a narrated transition table (move,
from→to, who can (guard→roles), when (precondition), help); **What happens
automatically** (automations matching this type); **Rules** (validations).
3. **Roles & permissions** (operator section) — narrative role table +
per-entity-type capability matrix (roles × CRUD + guarded transitions).
4. **Appendix: schema reference** — the dry enumerations.

**B2. Pure reference dump** — tables only, no narrative/lifecycle prose. Simpler
generator but ignores the Diátaxis insight; the doc-fields would be underused.
Rejected.

### Phase split

**C1. Additions first, then generator** (decided by user; recommended). Phase-1
ticket(s) add the four fields (+ populate them in the in-tree example projects
so the generator has real content to render). Phase-2 ticket builds `rela docs`.
Clean dependency: the generator renders a complete surface on day one.

## Recommendation

**A1 + B1 + C1.** Add the four minimal doc-fields first (metamodel top-level
description, CustomType per-value descriptions, TransitionDef help, ACL RoleDef
description), then build `rela docs --output-dir` emitting one Diátaxis-informed
Markdown file: reference tables as the backbone, the new prose fields supplying
the explanation layer, mermaid state diagrams + narrated transitions for
lifecycle, and a GitLab-style role×entity capability matrix for the operator
audience.

**Key tradeoff accepted:** the generator produces excellent *reference* and
decent *how-to* (lifecycle/automations), but never *tutorial* or deep
*explanation* — those remain human-authored. The four doc-fields inject just
enough explanation into the reference scaffold to make it genuinely end-user-
and operator-useful without turning the metamodel into a CMS. Starting minimal
(A1, not a `docs:` block) keeps the schema clean; we extend only if real output
reads thin.

**Suggested ticket breakdown:**
- TKT (Phase 1a): metamodel doc-fields — top-level description, CustomType
per-value descriptions, TransitionDef help. Populate in-tree examples.
- TKT (Phase 1b): ACL RoleDef description (+ optional policy description).
- TKT (Phase 2): `rela docs --output-dir` generator (depends on 1a+1b).

---

## ADDENDUM (2026-07-21) — reframed: from static generator to a **doc language**

Phase 1a/1b shipped (TKT-0YBFT8, TKT-DUQBD0, TKT-JO2SAD). Before ticketing phase
2, a design session (prompted by studying **sourcehaven-bv/openvwr**'s
Playwright screenshot system for its ISMS manual) reframed phase 2. The
recommendation above still holds for *what content to emit*; what changed is the
**authoring model**. Superseding decision: phase 2 is **not** a push-generator
that emits a whole `.md`; it is a **doc language** — markdown authored by a
human, with the mechanical fragments *pulled* from the schema/graph at build.

### The inversion (push → pull)

- **Old (B1):** `rela docs` walks the schema and emits one big Markdown. Prose
is either absent (dry) or robotic. Fragments can't be placed where a writer
wants them.
- **New:** the writer authors a normal Markdown manual and **escapes into Lua
islands** to summon fragments. Prose stays prose; reference fragments are live
includes that can't drift. This is Diátaxis done right — *explanation/how-to* is
authored, *reference* is resolved.

### The language — markdown with Lua islands (PHP/JSX/MDX model)

Markdown by default; two island forms (chosen syntax):
- **Statement island** — a fenced ` ```rela ` block. Runs Lua for
side-effects; doc-API calls append to an output buffer at that position (PHP
`echo ` model). For multi-emit / loops.
- **Echo island** — an inline `` `rela <expr>` `` span. Substitutes the string
value mid-sentence (interpolation).

Rationale for Lua-as-engine over a bespoke `{{directive}} ` grammar: rela
**already has** a Lua runtime (`internal/lua `, `ReadDeps `/`WriteDeps `),
the MCP `lua_* ` tools, and `just docs ` is already a Lua content pipeline.
The high-level doc API **is** the DSL — a *library* of functions bound into a
doc runtime, not a new language. The escape hatch (loops, conditionals) and the
common case are the same language, so the power tier costs zero extra grammar.
NOTE (validated in the prototype): the markdown host earns its keep because
manuals are ~85% prose — NOT because `{{directive}} ` reads better than `fn{}
` (it doesn't; that was an overclaim). The host format is the value, not the
sugar.

### The doc API (island functions) — validated against the ISMS corpus

Hand-resolved a real ISMS "Risicobeheer" chapter against
`sourcehaven-bv/isms-sourcehaven `'s `metamodel.yaml ` + live `RISK-001.md `
(prototype artifact built this session). Surface:

| Function | Kind | Reads | Emits |
| --- | --- | --- | --- |
| `typeref{type, fields} ` | resolver | metamodel | field/relation reference table |
| `values{type, field} ` | resolver | metamodel | enum values + per-value meaning (phase-1a `descriptions `) |
| `relations{type} ` | resolver | metamodel | flat relation list |
| `graph{from, depth, exclude/only, direction} ` | resolver | **`tracer `** | **mermaid subgraph** (see below) |
| `lifecycle{type, field} ` | resolver | metamodel | mermaid `stateDiagram-v2 ` (phase-1a `TransitionDef `) or flat-list fallback |
| `entity{id, fields} ` | resolver | **live graph** | one entity's values (worked example) |
| `count{type} ` | resolver | live graph | a number (echo) |
| `roles_matrix{type} ` | resolver | acl | GitLab-style role×entity capability rows |
| `description() ` | resolver | metamodel/acl | top-level deployment prose |
| `create `/`update `/`link ` | **seed (write)** | — | writes the throwaway memstore (see two-graph) |
| `screenshot{...} ` | **Tier B** | seed graph | captured, annotated PNG (see below) |
| `h1/h2/h3/md(...) ` | emit | — | structural markdown from Lua |

### Three design refinements from the session (each SIMPLIFIED the design)

1. **`graph{} ` = tracer + mermaid, with `depth ` + `exclude `/`only `.** Thin
renderer over the existing `tracer ` package (bounded walk + edge filters).
`from ` as a **type** draws the schema neighbourhood (metamodel read); `from `
as an **id** traverses the live graph (`tracer `). `exclude ` (prune plumbing
edges like `spawnt `/`gaat_over `) is what makes the diagram *publishable* —
turns a hairball into the domain story. `only ` is the allowlist inverse. Node
labels get the phase-1a.5 injection-safe treatment (`mermaidLabel `/synthetic
ids).

2. **No `seed.* ` API — reuse the EXISTING rela Lua write bindings.** The seed
for a screenshot is just `create `/`update `/`link ` (the same `WriteDeps `
API a scheduler script uses), bound to the throwaway memstore instead of the
real store. So the **two-graph model IS the CLAUDE.md `ReadDeps `/`WriteDeps `
split**: read bundle → the documented (real) project, read-only; write bundle →
a fresh ephemeral `memstore `, discarded post-build. Writes go through
`entitymanager `, so fixtures are valid by construction. This is the sanctioned
place the "no user Lua on the read path" rule does NOT bite (offline operator
build; writes land in a throwaway). It also cleanly resolves the earlier "schema
vs live-graph" fork: `entity{id="RISK-001"} ` reads real data;
`screenshot{entity=r.id} ` reads fixture data — different bindings, visibly
different graphs.

3. **Screenshot annotations = arrows-with-text (openvwr's actual vocabulary).**
`arrows = {{at, text, side}, ...} ` — the label rides the shaft (the common
case). `at ` targets a **field** (`"score" ` → schema-checked `data-field `,
fail-loud) OR a **control** (`"@button:opslaan" ` / `"@role:..." ` → ARIA).
Keep openvwr's `box `/`badge `/`redact ` as secondary primitives. rela beats
openvwr here: the data-entry form IS metamodel-generated, so anchors are schema
fields the harness knows exist — no brittle `getByRole() ` CSS; a renamed field
breaks the build instead of drifting.

### The Tier A / Tier B split (drives the phase-2/phase-3 boundary)

| | **Tier A — resolvers** | **Tier B — `screenshot{} `** |
| --- | --- | --- |
| Reads | metamodel + graph + tracer (`ReadDeps `) | drives the live data-entry SPA |
| Needs | nothing — pure, offline | Playwright + seeded memstore + role auth |
| Output | inline markdown / mermaid | PNG + `![]() ` reference |
| Runs on | any laptop, CI, anywhere | CI (or opt-in dev) only |
| Failure | **fail-loud** on bad schema ref | **degrade**: placeholder + warning, build still succeeds |

**Tier B must degrade, never block:** a manual always builds without a browser
(full text/tables/mermaid; screenshots become placeholders). The memstore seed
itself is browser-free and always runs, so even a degraded placeholder is
accurate about what it would show.

### openvwr — what was studied and adopted

openvwr builds its ISMS manual via **pandoc + Eisvogel → PDF** from hand-written
markdown chapters; figures are auto-generated by a **Playwright** `FIGURES
`-list harness (`tools/screenshots/ `), selectors anchored to ARIA roles not
pixels, annotations (`arrow `/`box `/`badge `/`redact `) anchored to
selectors and **fail-loud** if an element is missing. Adopted: the fail-loud
discipline, the arrow-with-text annotation vocabulary, and the "generated
reference + hand- written narrative, pandoc-able to PDF" two-track shape.
Improved on: the figure list is not hand-maintained — it's inline islands; and
anchors are schema fields, not CSS.

### Open questions to settle in the phase-2 ticket (none structural)

1. **Function vocabulary** — lock a tight, consistent naming set before it
ossifies (`typeref `/`values `/`relations `/`graph `/`lifecycle `/`entity
`/`count `/ `roles_matrix `/`description `/`create `/`link `/`screenshot
`).
2. **Empty-resolve strictness** — an island resolving to nothing (e.g.
`description() ` when no top-level description exists): silent `"" ` vs warn
vs fail. Needs a per-build strictness knob (openvwr fails loud).
3. **Island marker** — ` ```rela `/`` `rela ` `` chosen (raw source renders as
a fenced code block on GitHub rather than garbage; greppable). Preprocessor runs
before any markdown renderer sees the file.
4. **Live-graph reads in a manual** — `entity{} `/`count{} ` embed real instance
data. Powerful (can't go stale) but may warrant an opt-in per build. Distinct
from seed/screenshot data (memstore).

### Revised phase split (supersedes the "Suggested ticket breakdown" above)

- **Phase 1a / 1a.5 / 1b** — doc-fields. DONE (TKT-0YBFT8, TKT-DUQBD0,
TKT-JO2SAD).
- **Phase 2 = Tier A** — the doc language + all schema/graph resolvers +
markdown-island preprocessor, read-only against the real project (+ the memstore
seed *write* wiring, since `graph `/`entity ` share the runtime and the seed
is the same write API). Emits markdown; pandoc-able to PDF. NO browser.
- **Phase 3 = Tier B** — the `screenshot{} ` island: metamodel-driven Playwright
harness against the seeded memstore, arrow annotations, degradation. Depends on
phase 2 + needs the data-entry server to accept an in-memory project handle at
build time (the `memorybackend ` build tag proves the server can run on
memstore — plumbing, not new capability).
