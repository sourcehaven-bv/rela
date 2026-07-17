---
id: RES-H5AB7S
type: research
title: 'Enforce property-level visible: redaction across all read surfaces (GET, list, ?include=, /_position, /_search, MCP)'
summary: 'DataEntry REST already redacts visible: properties via the serializer; the real gaps are MCP (zero read ACL — reuse the affordances resolver) and the search match-on-hidden-field oracle (drop hits at the VisibleSearcher seam).'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Problem

Property-level visibility (`visible:` grants in `acl.yaml`) hides individual
**properties within an otherwise-readable entity** — distinct from entity-level
`read:` which drops the whole entity. The question: **is `visible:` redaction
enforced on every read surface, and if not, which surfaces leak, and how do we
close them without re-deriving the verdict per-row?**

A load-bearing sub-question the original framing raised: a full-text **search
hit that matched only on a hidden field** must be **dropped**, not merely
body-redacted. If the entity still surfaces (because the index holds the hidden
field's text), `q=<candidate postcode>` becomes a present/absent oracle — search
degrades into an address oracle even though the response body omits the field.

## Context

### Reality check — the gap is narrower than the security doc states

`docs/security.md:371-373` says *"Field visibility via `visible:` omits a
property from the **write-form** response, but list-query reads are not
filtered."* **This is now stale.** The serializer applies redaction to all
data-entry v1 REST surfaces:

- `entitySerializer.forWire` (`internal/dataentry/entityserializer.go:66`) →
`stripHiddenProperties` — per-entity GET, POST, PATCH, clone, action.
- `entitySerializer.forWireRelated` (`:78`) → `stripHiddenProperties` — **list
rows, `?include=` peers, and `/_search` results** (`api_v1.go:1623`, `:1814`,
`:321`).
- `stripHiddenProperties` (`affordances.go:846`) deletes hidden keys from
`result.Properties` **and** falls the `_title` back to the entity ID when the
primary/display property is hidden — so the title doesn't leak a redacted value
either.

The verdict source is real, not stubbed: `affordances.PolicyResolver`
(`internal/affordances/resolver.go`) compiles the `visible:` block (with `when:`
predicates) at startup and returns `FieldVerdicts.Visible` per (principal,
entity). The dataentry adapter is `policyResolver` (`affordances_policy.go`).

### The actual gaps

1. **MCP has zero read ACL.** `internal/mcp/convert.go:69` serializes
`Properties: e.Properties` raw. `show_entity`, `list_entities`,
`search_entities`, and the trace tools (`tools_entity.go`, `tools_trace.go`)
call `store.GetEntity` / `Searcher.Search` directly — **no entity-level read
gate and no property redaction.** This is the dominant hole: an agent gets full
bodies including `visible:`-hidden fields. (Entity-level MCP gating is itself
still open — see the `authorization` concept's MCP follow-ups — so property
redaction on MCP rides on, or stacks with, that work.)

2. **The search match-on-hidden-field oracle.** `internal/search/filter.go:94`
indexes **every** property value (`for _, v := range e.Properties`). So even on
the already-redacted `/_search`, a query whose only match is a hidden field's
text still returns the entity as a hit. Body redaction (`api_v1.go: 1623`) hides
the *value* but not the *presence* — the oracle survives. `executeQuery`
(`helpers.go`) gates at entity level only; it has no notion of "this hit matched
a field this principal can't see."

3. **`/_position` (minor).** Returns only `{prev,next,current,total}` with
IDs/types (`scope.go:202`) — no property values, and entity-level gated via
`resolveScope`. The only residual: a caller-supplied `scope` could **sort or
filter on a hidden field**, a weak ordering-oracle. Low severity; flag, don't
necessarily fix in v1.

### Patterns to reuse (don't reinvent)

- **`readGate` interface** (`internal/dataentry/readgate.go:44`) — the
consumer-side seam. Entity-level methods (`PermitsRead`, `PermitsReadMany`,
`ReadQuery`, `SearchScope`) already exist; a property-redaction method is the
natural fifth.
- **`visibleReader`** (`internal/dataentry/visiblereader.go`) — the structural
choke point: holds the store privately, exposes only gated reads, makes gating
structural rather than by-convention. The exact place property redaction belongs
(it already drops whole entities; redaction is the within-entity analog).
- **Per-request verdict caching** — `acl.Request` memoises `GlobalRoles`
(`request.go:57`); the affordance resolver is invoked once per entity per
request. A per-(principal,type) **redaction-fieldset** cache is the same pattern
as the existing read-query memoisation.
- **`VisibleSearcher` seam + `storetest.RunVisibleSearchTests`**
(`internal/store/storetest/visiblesearch.go`) — 15 cases,
`VisibleSearchFactory`, no-leak + ordered-subsequence invariants, run ×3
backends (generic+memstore, generic+bleve, pgstore-native DB-gated). The exact
template for a field-visibility conformance suite.

### Constraints (from CLAUDE.md and the ACL arc)

- **No Lua on the read path** (`internal/entitymanager/CLAUDE.md`,
`authorization` concept). `when:` predicates compile to the gopher-lua *subset*
evaluator already used by affordances — that's allowed; arbitrary user Lua is
not.
- **Consumer-side interfaces, narrow.** A redaction method belongs on the
consumer's `readGate`, not a producer god-interface.
- **NopACL byte-parity.** Under no `acl.yaml`, every read surface must be
byte-identical to today (the nop resolver returns empty `Visible` → no-op).
- **Predicates evaluate against the *full* entity**, including soon-to-be-hidden
fields (intentional — a visible field's verdict may key on a hidden one).
Redaction happens *after* verdict computation.
- **Fail-closed.** A resolver/predicate error must hide the field, never reveal
it (mirrors `visibleReader.filterVisible`'s drop-the-type-on-error).

## Options

The two gaps are largely independent and can ship as separate tickets.

### Gap A — MCP property redaction

#### Option A1: Reuse the affordances resolver behind an MCP read seam

- **Approach**: Wire the `affordances.PolicyResolver` (or a narrow
`FieldRedactor` interface over it) into the MCP `Services` bundle. Add a
`redactProperties(ctx, e)` choke point in `convert.go` that every
entity-returning tool routes through — symmetric to dataentry's
`stripHiddenProperties`. Stack on / share the entity-level MCP read gate from
the `authorization` MCP follow-ups so a denied *entity* never reaches the
redactor in the first place.
- **Pros**: One verdict implementation across HTTP + MCP (no policy drift);
predicates already compiled; the choke-point shape is proven in dataentry.
- **Cons**: Pulls the affordances resolver (currently dataentry-adjacent) into
the MCP wiring; needs a principal on the MCP ctx (the MCP transport already
carries `Principal{User,Tool}` — `internal/mcp/principal.go`). Couples to the
still-open MCP entity-gate work.
- **Effort**: ~1–1.5 days once a principal + entity-gate are on the MCP read
path; the redaction layer itself is small.

#### Option A2: MCP-local redaction copy

- **Approach**: Reimplement the strip inside `internal/mcp/` against
`acl.Policy` directly.
- **Pros**: No cross-package wiring.
- **Cons**: **Policy drift** — two redaction implementations to keep in sync;
re-compiles or re-walks predicates. Violates "one verdict source." Not
recommended.
- **Effort**: ~1 day, but creates a maintenance liability.

### Gap B — search match-on-hidden-field oracle

#### Option B1: Per-principal candidate re-rank, drop hits that match only hidden fields (generic seam)

- **Approach**: Extend the `VisibleSearcher` contract: after entity-level
scoping, for each surviving hit determine the **fields the principal may see**
and re-evaluate whether the query still matches on a *visible* field. If the
match set ⊆ hidden fields → drop the hit. Generic impl re-checks candidate field
text in-process (bounded by the existing candidate window); pgstore-native
builds the visible-field projection into the trgm/tsvector predicate so the
match is computed over visible columns only.
- **Pros**: Closes the oracle at the seam; extends the conformance suite that
already exists; same generic-vs-native split as `TKT-BA8BSX`.
- **Cons**: pgstore pushdown is real work (per-principal visible-field set →
column projection in the WHERE); generic impl must hold candidate field text to
know *what* matched. Field-visibility verdicts can be `when:`-predicate
dependent (per-entity), so the visible-field set isn't always static per
(principal,type) — the predicate-dependent case may force per-row evaluation
(bounded, but not pushdown-friendly).
- **Effort**: ~1 day generic + drop logic; ~1–1.5 days pgstore pushdown for the
static (predicate-free) case; predicate-dependent fields fall back to
per-candidate eval.

#### Option B2: Per-principal index partitioning / field-scoped index

- **Approach**: Maintain separate index views excluding hidden fields per role.
- **Pros**: Match never sees hidden text; fast at query time.
- **Cons**: Combinatorial in roles × predicate-conditioned visibility; index
bloat; rebuild churn. Over-engineered for the threat. Not recommended.
- **Effort**: Multi-day, high maintenance.

#### Option B3: Document-and-defer the oracle, redact bodies only (status quo +)

- **Approach**: Accept that body redaction is in place; treat the oracle as a
documented residual for v1 and gate it behind a follow-up.
- **Pros**: Zero new code; honest about the residual.
- **Cons**: Leaves the exact attack the original framing called out (postcode →
address oracle). Only acceptable if `visible:` fields are not used for guessable
secrets in practice.
- **Effort**: Doc only.

## Recommendation

**Two tickets, in priority order:**

1. **MCP redaction (Option A1)** — highest impact and least subtle. MCP today
returns *raw* bodies with no read ACL at all; an agent reads everything.
Implement property redaction as a `convert.go` choke point reusing the single
`affordances`/`acl` verdict source, stacked on the entity-level MCP read gate
from the existing `authorization` MCP follow-ups (sequence after, or jointly
with, that work). Conformance: an MCP-level analog of the no-leak invariant
(hidden property absent from every tool's output).

2. **Search oracle (Option B1)** — close it at the `VisibleSearcher` seam,
extending `storetest.RunVisibleSearchTests` with field-visibility cases (hit
that matches only a hidden field → dropped; same no-leak + ordered-subsequence
invariants, now over the visible-field projection). Accept the pushdown
limitation honestly: **static** (predicate-free) `visible:` field sets get
pgstore column-projection pushdown; **predicate- conditioned** visibility falls
back to bounded per-candidate evaluation within the existing candidate window.
`log()` the fallback so the bound is visible, mirroring the bleve-10k-floor
disclosure in `TKT-BA8BSX`.

**Tradeoffs accepted:**

- We do **not** rebuild the security doc's "write-form only" claim — we
**correct** it: dataentry REST already redacts; the doc is stale and should be
updated as part of (1).
- `/_position` sort/filter-on-hidden-field is a documented low-severity residual
(Option B3 for that surface only) — not worth pushdown complexity.
- Predicate-conditioned field visibility is not fully pushdown-able; bounded
per-candidate eval is the accepted cost, consistent with the existing seam's
candidate-window bound.

**Performance contract** (as the original framing asked): the redaction fieldset
is cached per (principal, type) per request — the same memoisation shape
`acl.Request` already uses for `GlobalRoles` — *except* where a `when:`
predicate makes visibility entity-dependent, in which case the verdict is
computed per entity (already the affordance resolver's per-entity contract).
pgstore column-projection pushdown applies only to the static-verdict case.
