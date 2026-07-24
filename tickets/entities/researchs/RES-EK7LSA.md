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
