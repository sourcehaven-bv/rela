---
id: RES-PSZZKU
type: research
title: 'ACL-bound read seam for Lua + tracer: a shared row-gating, field-redacting visibility.Reader enabling scheduled ACL-bound LLM jobs'
summary: 'Hoist row-gate + field-redaction into a shared internal/visibility.Reader (PolicyReader + AllowAllReader) that data-entry and Lua read through; keep tracer pure and gate its nodes at the seam (withhold paths through hidden nodes); system jobs get a genuine principal + allow-all reader (capability, not identity). Ship as a 3-PR arc: seam, export fix, Lua reads.'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Problem

An export path in PR #1188 (transform registry) fed an entity's **raw
properties** into an external converter, bypassing the field-level ACL redaction
(`visible:` grants) that every data-entry REST surface already enforces. The
CISO IB-review blocked the PR: export must not leak a field the requesting
principal cannot see.

The narrow fix is three missing redaction calls. But the finding exposes a
**structural** gap: field-redaction (and, below it, entity-row gating) is
enforced **by convention at each consumer**, not by the read seam itself. Any
new consumer that reads entities and renders/serializes them re-opens the same
hole — export did. And the gap is widest exactly where we want to grow:

- **Lua scripts read a raw `store.Store`** — no row-gate, no redaction. Every
property-bearing binding (`get_entity`, `list_entities`, `search`,
`get_relations`) emits full `Properties`.
- **The tracer is a pure reader** with zero ACL awareness; its Lua/MCP/CLI
consumers expose traversal output ungated.

The motivating goal (user's): **schedule ACL-bound LLM jobs.** A scheduled Lua
job that reads the graph, stuffs content into a prompt, calls an LLM provider,
and writes back derived entities is the confused-deputy threat made unattended
and injectable. Containing what such a job can **read** — row-gated and
field-redacted to a job role — is what makes it safe to schedule at all: what
never enters the reader never enters the prompt, never reaches the vendor, never
lands in a written summary. This research asks: **should read-side ACL (row-gate
+ field-redaction) move into a shared seam above the store that data-entry AND
Lua both read through, and if so, what is its shape, how far into tracer
traversal does it reach, and how do scheduled/system jobs opt into allow-all
without lying about their identity?**

Write-side is explicitly **out of scope**: `entitymanager` already owns
write-ACL, the audit log, and attribution (`Deps.ACL` required; `rela.principal`
read-only; scheduler stamps `ToolScheduler` + `triggered_by`). This is a
read-containment arc.

## Context

### Sibling prior art — what is already decided

- **[[RES-H5AB7S]]** (done) covered property-`visible:` redaction across the
**REST / MCP / search** surfaces. It confirmed **data-entry REST already
redacts** via the serializer choke point (`entitySerializer.forWire*` →
`stripHiddenProperties`), corrected the stale `docs/security.md` "write-form
only" claim, and scoped its remaining gaps to **MCP** (`convert.go` returns raw
bodies) and the **search match-on-hidden-field oracle** (→ TKT-GGQ0JT,
TKT-L59KUM). It **explicitly did not touch Lua or tracer.** This research is its
sibling: the **Lua + tracer** read surfaces, triggered by the export leak.
- **[[TKT-Z1OP7R]]** (backlog) is the **egress / capability** half of a safe
LLM job — `http.*` on the read path, SSRF, local-service egress: where a script
may **send** data. This research is the **read-visibility** half: what data
**enters** the script. Both are prerequisites for the ACL-bound LLM job; they
compose (contain the input; contain the output).
- **TKT-QU7REX** (done) established the enforcement pattern: data-entry
`_analyze` runs whole-graph scans **ungated**, then filters the resulting issue
list at the HTTP boundary via `readGate.PermitsReadMany`
(`visibleAnalysisIssues`). The analyze service doc says so explicitly:
*"whole-graph scans are intentionally ungated; visibility is applied to the
resulting issues at the HTTP boundary."* → Precedent for **keeping the tracer
pure and gating at the seam/consumer**, not inside `internal/tracer`.
- **TKT-D8T148** established the capability-object precedent:
`WriteDeps.ElevatedManager` — elevation is the **presence of a capability
object** set at exactly one wiring site, absent (nil) everywhere else, which is
what makes `rela.bypass_acl` available. The allow-all reader for system jobs is
the read-side analog of this.
- **TKT-73C6B2** (freeze field-visibility verdict at version capture) is an
adjacent redaction-correctness concern; out of scope here but noted so history
redaction stays consistent with whatever verdict source this arc uses.

### Reusable machinery (both pieces are already dataentry-agnostic)

The enabling fact: the general row-gate and the general field-resolver **already
live in shared packages**; data-entry only adds thin package-private adapters
over them. A shared seam hoists the *adapters*, not the *engine*.

- **Row-gate** — `acl.Request` (`internal/acl/request.go`):
`PermitsRead(ctx,type,id)`, `PermitsReadMany(ctx,type,ids)`,
`ReadQuery(ctx,type)`. Built from a principal via
`Declarative.ForPrincipal(principal.From(ctx))` (rejects unstamped principals →
`ErrUnstampedPrincipal`); amortized on ctx via `acl.WithRequest` /
`acl.FromContext`. `acl.mayDependOn: entity, principal, store`.
- **Field-redact** — `affordances.PolicyResolver.FieldVerdicts(ctx,e)`
(`internal/affordances/resolver.go:351`) → `FieldVerdicts.Visible
map[string]bool` (**sparse: absent = visible; explicit `false` = hidden**).
Constructed `affordances.New(meta, lookup, declarative)`. **Does not import
dataentry** (one-way edge, asserted in its package doc).
`affordances.mayDependOn: acl, entity, metamodel, predicate, principal,
statemachine`.
- **Data-entry local adapters** (the shape to hoist): `readGate` +
`aclReadGate`/`nopReadGate` (`readgate.go`); `visibleReader.getVisible` /
`filterVisible` (`visiblereader.go` — "a consumer that takes a visibleReader
cannot bypass gating"); `policyResolver` + `storeRelationLookup`
(`affordances_policy.go`); `hiddenProperties` / `stripHiddenProperties` /
`copyVisibleProperties` (`affordances.go`). `stripHiddenProperties` also
**rewrites the title to the ID when the display property is hidden** — a
secondary-channel leak the export renderer's `DisplayTitle` would otherwise
re-open.

### Arch-lint verdict — a new `internal/visibility` package is legal, no cycle

`.go-arch-lint.yml` (whitelist model, transitive-reachability checked):

- New component `visibility.mayDependOn: acl, affordances, store, entity` —
introduces **no cycle** (nothing in acl/affordances/store imports back).
- `lua.mayDependOn` **currently lacks `acl` and `affordances`** (confirmed: lua
imports neither). Adding **`visibility`** to `lua.mayDependOn` transitively
authorizes the reach — one entry, no cycle. Same one-entry add for
`dataentry.mayDependOn` (dataentry already whitelists acl+affordances).

### The Lua read surface (what redaction must actually touch)

9 read bindings; the choke point is `EntityToTable` (`runtime.go:1080`), which
copies **every** property verbatim.

- **Property-bearing (need redaction):** `get_entity`, `list_entities`,
`search` (re-fetches the **full** entity per hit via `Store.GetEntity`),
`get_relations`. All route through `EntityToTable` / `relationToTable`.
- **Already Property-free by shape (need only row-gating):** `trace_from`,
`trace_to`, `find_path` — `traceResultToTable` carries id/type/title/relation
only, no Properties. **So tracer bindings need node-drop, not redaction.**
- **Schema-only (no gating):** `get_entity_types`, `get_relation_types` (read
`Meta`).
- Every read flows through **one seam**: `callerCtx()` (`runtime.go:167`).

### The principal-threading gap (reshapes the export fix)

- `execute()` / action paths pass `WithPrincipal(principal.From(ctx))`.
- **`ExecuteDocument` (the `export_render:` path — the exact PR #1188 surface)
takes no ctx and runs Lua reads on `context.Background()` — NO principal.** MCP
`lua_eval`/`lua_run` also omit `WithPrincipal` (principal reaches reads only via
`callerCtx`, not `rela.principal`).
- ⇒ Redacting the export path **requires first threading the request principal
into document render.** Without a principal on ctx, `Declarative.ForPrincipal`
rejects it — a redacting reader would fail-closed (deny-all), which is safe but
breaks legitimate export until the principal is wired.

### The scheduler / system-job identity (validates the capability approach)

`scheduler.stampTaskAuditContext` already sets `principal.With(ctx, {User:
SystemUser(), Tool: ToolScheduler})` +
`audit.WithTriggeredBy("schedule:"+name)`. **Honest system identity already
exists** and is audited. So allow-all for a job is a **reader capability**
(which reader it was handed), never an identity hack — identity stays truthful
for the audit log, exactly the `ElevatedManager` shape.

### Constraints

- **Not in `store.Store`** (CLAUDE.md: depend directly on store; store is
ACL-unaware; tracer/search/entitymanager/validator all need raw reads).
- **Fail-closed**: resolver/gate error hides the row/field, never reveals
(mirrors `filterVisible`'s drop-on-error).
- **NopACL byte-parity**: no `acl.yaml` ⇒ nop reader ⇒ byte-identical to today.
- **Predicates evaluate against the full entity** (a visible field's `when:` may
key on a hidden one); redact *after* verdict.
- **Behavior change to note**: a data-entry-invoked Lua script (export_render,
MCP) will read the **caller's redacted view** after this — intended (it is the
fix), but a change for scripts that assumed full reads. CLAUDE.md's "no user Lua
on the read path" targets unbounded per-row predicates, not this bounded seam.

## Options

Two independent axes: **(1) where the shared seam lives** and **(2) how far into
tracer traversal gating reaches**. Plus the **system-job capability** decision,
which is settled.

### Axis 1 — the shared seam

#### Option 1A: New `internal/visibility` package with a `Reader` seam (recommended)

A leaf package depending on `{acl, affordances, store, entity}`, exposing:
```go
type Reader interface {
    Get(ctx, entityType, id) (*entity.Entity, bool, error) // row-gate THEN field-redact (copy)
    Filter(ctx, candidates)  []*entity.Entity              // row-gate + redact each
    // list/query scope (ReadQuery) can join later
}
```

Two impls: `PolicyReader` (composes `acl.Request` + `affordances` from ctx
principal; returns a **copied**, redacted entity, incl. the title-fallback) and
`AllowAllReader` (pass-through, no gate/redaction). `dataentry.visibleReader`
becomes a thin wrapper (or is replaced); `lua.ReadDeps.Store store.Store` →
`ReadDeps.Reader visibility.Reader`.

- **Pros**: One enforcement seam, structural not by-convention — export can't
re-open the hole; hoists proven adapters; no cycle; the `AllowAllReader` /
`PolicyReader` split is the clean home for the job-capability decision; extends
naturally to MCP later (closes RES-H5AB7S's MCP gap with the same seam).
- **Cons**: Touches lua's core dep bundle (a real refactor across every wiring
site and every read binding); the seam must be **read-out only** — the
write-prep read (entitymanager computing a diff) must NOT go through a redacting
reader or it would clobber hidden fields on save (so it can't blanket -replace
every `store.Store` use). Interface design must stay narrow (consumer-side
rule).
- **Effort**: L. New package + 2 impls + conformance suite; rewire lua ReadDeps
  + every binding through the reader; rewire export/list; thread principal into
`ExecuteDocument`; arch-lint edits.

#### Option 1B: Narrow interfaces, no new package — each consumer wires acl+affordances itself

Give `lua` its own local `readGate`/`redactor` interfaces (à la dataentry) and
wire the acl/affordances impls at each site.

- **Pros**: No new shared package; maximal consumer-side-interface purity.
- **Cons**: **Re-creates the by-convention problem** — two (soon three, with
MCP) parallel adapter sets to keep in sync; the exact drift RES-H5AB7S and this
finding warn against. Rejected.
- **Effort**: M, but higher long-run maintenance.

#### Option 1C: Point-fix export only (status quo + 3 calls)

Redact at the three export call sites; leave Lua/tracer raw.

- **Pros**: Unblocks the PR in hours.
- **Cons**: Does nothing for the LLM-job goal; leaves Lua a raw-read hole; the
next renderer re-opens it. The user explicitly rejected temp hacks.
- **Effort**: S. Not recommended as the arc; may be the PR-level stopgap only if
the arc must be sequenced later.

### Axis 2 — tracer traversal depth (the hard part)

Tracer returns full `Properties` in-memory (`json:"-"`, consumed by
`internal/output`) and has **no** hidden-node-mid-path handling anywhere.

#### Option 2A: Gate tracer at the seam/consumer, tracer stays pure (recommended)

Keep `internal/tracer` ACL-free. The `visibility` seam / each consumer row-gates
**the nodes of the returned tree/path** (drop hidden nodes; for a path, a hidden
intermediate node means the path is **withheld** — fail-closed — since revealing
"a path exists through X" is itself a leak). Redaction is moot for Lua trace
bindings (already Property-free); titles still need gating (a hidden node's
title must not surface).

- **Pros**: Matches the established TKT-QU7REX precedent (whole-graph scan
ungated, filter results at boundary); keeps tracer a pure reader per CLAUDE.md;
bounded work.
- **Cons**: Post-hoc node-drop on a tree is straightforward, but **paths need a
policy decision**: drop the whole path vs. return "no path" vs. return a
truncated path. Withhold-on-hidden-intermediate is the safe default but changes
find_path semantics under ACL.
- **Effort**: M.

#### Option 2B: ACL-aware tracer (gate mid-traversal)

Push a read-gate into traversal so hidden nodes are never walked.

- **Pros**: No hidden node ever materialized; a path genuinely routes around
hidden nodes.
- **Cons**: Violates "tracer is a pure reader" (CLAUDE.md); pulls acl into
`internal/tracer` (arch-lint + conceptual regression); per-node gate calls in a
hot traversal (perf); "route around hidden node" can itself leak topology.
Rejected for this arc.
- **Effort**: L, higher risk.

#### Option 2C: Defer tracer, ship entity/list/search gating only

Scope this arc to the property-bearing reads (`get_entity`/`list`/`search`/
`get_relations`) + export; leave trace/path bindings ungated with a documented
residual + follow-up ticket.

- **Pros**: Smaller, lands the LLM-job-relevant 80% (the property leak) fast;
trace bindings are already Property-free so the residual is *titles + topology*
only, not field values.
- **Cons**: Leaves a title/topology leak on trace/path under a real principal;
an LLM job doing "find everything about X" via trace stays ungated.

### Axis 3 — system-job capability (settled)

Jobs run under a **genuine `system:*` principal** (honest audit — already
implemented) **and** are handed an **`AllowAllReader`** at the wiring site
(capability, not identity). `PolicyReader`-for-jobs (a role-scoped reader, the
enabler for ACL-bound LLM jobs) is supported by the same seam and is the point
of the arc; whether existing scheduler jobs default to allow-all
(non-regressing) or scoped is a per-job wiring decision, with allow-all as an
explicit, visible opt-in — never the silent default.

## Recommendation

**Option 1A (new `internal/visibility.Reader` seam) + Option 2A (tracer stays
pure, gate nodes at the seam) + Axis 3 as settled.** Sequenced as a small arc,
not one PR:

1. **PR 1 — the seam.** New `internal/visibility` package: `Reader` interface,
`PolicyReader` (row-gate + field-redact + copy + title-fallback),
`AllowAllReader`; conformance suite (row-gate + redaction + title-fallback +
fail-closed + NopACL byte-parity), modeled on `storetest.RunVisibleSearchTests`.
Arch-lint edits. No consumer rewired yet.
2. **PR 2 — data-entry export (closes the #1188 finding structurally).** Route
entity export, list export, and the `export_render:` entity input through the
reader. **Thread the request principal into `ExecuteDocument`** (the
prerequisite the Lua map surfaced). Negative tests: hidden field never in
PDF/list/title.
3. **PR 3 — Lua reads.** `ReadDeps.Store` → `ReadDeps.Reader`; route the 4
property-bearing bindings through the reader; row-gate the 3 trace bindings'
nodes (withhold-path-on-hidden-intermediate). Wire **existing scheduler jobs to
`AllowAllReader`** (non-regressing). Prove **one** `PolicyReader`-scoped job
end-to-end (test/example) so the ACL-bound-LLM-job path is demonstrated, not
just theoretical.
4. **Follow-ups (tickets, not this arc):** per-job role authoring UX; MCP reads
through the same reader (closes RES-H5AB7S's MCP gap); derivation/write-back
provenance governance (an LLM job writing a summary of visible-but-sensitive
data into a more-visible entity — read-containment is necessary, not
sufficient); compose with TKT-Z1OP7R egress controls for the full LLM-job safety
envelope.

**Tradeoffs accepted:**

- **Tracer stays pure**; paths through hidden intermediates are **withheld**
(fail-closed), a semantic change to `find_path` under ACL — the safe default.
- The redacting reader is **read-out only**; the write-prep read path keeps raw
`store.Store` access (redacting it would clobber hidden fields on save), so this
is not a blanket `store.Store` replacement.
- `ExecuteDocument`/MCP-lua currently lack a principal; wiring one is a
prerequisite, and until then those paths fail-closed (deny) rather than leak —
acceptable.
- Existing scheduler jobs stay allow-all (non-regressing); per-job scoping is
opt-in and lands as a follow-up — the architecture supports it from day one, but
we don't mandate policy authoring for every job in this arc.
