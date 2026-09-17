---
id: BUG-9Z20WH
type: bug
title: 'executeView traversal is ungated: hidden intermediaries leak reachable descendants in _views API, entity-detail sections, and view commands'
description: 'The view traversal walks the relation graph with no ACL read-gate, so a hidden node is traversed THROUGH; the _views API, entity-detail sections, and view commands all gate only the entry entity, then serve that traversal closure. A hidden intermediary H on Entry->H->V leaks the visible descendant V to a principal who could never read it directly. Fix: source-gate the recursive frontier and the collection load (plus a defensive entry gate) so a hidden node can neither enter the working set nor act as a stepping-stone. Gating only the load is NOT sufficient — V is readable in its own right, so it survives both the load gate and the out-filter.'
priority: high
effort: l
why1: The view traversal expands its walk with no ACL read-gate, so a hidden node is traversed THROUGH and its descendants collected; all three consumers then serve that result after gating only the entry entity. (Originally diagnosed as an ungated raw-store read in traverseViewOnce -> st.GetEntity. The viewsHandler refactor, TKT-1U8XYN, deleted that function and made the recursive walk id-only, so the defect survived in a sharper form — the walk loads no entities at all, and the ungated step is the FRONTIER EXPANSION rather than a read.)
why2: The traversal predates internal/visibility (DEC-ZBI39P). When the read-side ACL was decomposed into visibility decorators, every other read-out path was wrapped, but this traversal reader was not — it was missed, not deliberately excluded.
why3: There was no structural enforcement that a new read-out reader must go through a visibility wrapper; the rule ('read-out paths go through visibility wrappers') is a CLAUDE.md convention checked by review, not by a lint or a type boundary, so a pre-existing reader that predates the rule stays invisible to it.
why4: The read-side ACL decomposition (DEC-ZBI39P) modeled visibility as a property of rows on the way OUT, so every seam it introduced is a filter over a result set. Reachability is not a property of a row, it is a property of the walk — no seam in the design expresses 'this node may not be traversed through', so there was nowhere for the traversal to plug in even if someone had looked.
why5: Systemic — the project has a rule for read-out paths but no rule, type, or lint distinguishing a READ-OUT from a TRAVERSAL. Filtering a result set and gating a graph walk are different operations with different failure modes, and treating the second as an instance of the first is what makes this class of leak keep looking fixed when it is not (RR-3T18K9 already had to argue that result-filtering is not source-gating, and the same trap recurred in the refactored shape).
prevention: Added TestViewTraversalIsSourceGated (internal/dataentry/lint_test.go), a structural tripwire asserting BOTH source-gate sites — the recursive frontier in views.go and the collection load in viewworld.go. It deliberately has no 'guard is moot' skip branch — the first draft of this guard keyed a skip on views.go still containing GetEntity, and the very next refactor moved that read to viewworld.go, which would have silently disarmed the guard while reopening the hole. The broader systemic gap (why5) — that the codebase does not distinguish a read-out from a traversal — is recorded on the dataentry-readout-visibility-gate measure; a package-wide "no raw store reads" lint was rejected as needing a large, drift-prone allowlist (analyze tools, sync, write-prep diffing that must stay raw).
status: done
---

## Summary

The view traversal walks the relation graph with **no ACL read-gate**, and its
result is served to users by three surfaces. All three gate only the **entry**
entity, then serve the ungated traversal closure. A hidden intermediary `H` on a
path `Entry → H → V` leaks the visible descendant `V` to a principal who could
never read it directly — a read-side confidentiality leak.

Found during TKT-2FDTJE's design review (RR-3T18K9 / RR-WAE2E4), which split it
out because it affects surfaces beyond commands.

## The leak (verified)

The view traversal walks the relation graph with no read gate, so a hidden node
is traversed **through**.

On `Entry → H → V` with `H` hidden and `V` visible:

- a **source-gated** traversal stops at `H` → `{}`
- the **ungated** traversal → `{V}`

The presence of `V` (reachable only through `H`) leaks that `H` exists and links
`Entry` to `V`. Secondary: `applyViewTraverse` runs `filterEntities(found,
rule.Where)`, so collection membership is a function of hidden entities'
property values.

### Shape on current develop (revised)

The original write-up located the defect at `traverseViewRecursive` recursing
through `traverseViewOnce -> st.GetEntity`. **The `viewsHandler` refactor
(TKT-1U8XYN) deleted both functions**, and the defect survived in a sharper
form. The traversal now splits into:

- **id-collection** — `traverseViewMany` / `traverseViewBreadthFirst`, which
walk the graph on **ids alone and load no entities**; and
- **one batched load** — `loadViewEntities`, once per rule application.

So the ungated step is no longer a read, it is the **frontier expansion**: the
BFS reaches `V` via `H`'s id without ever materializing `H`. This is why the fix
cannot be a gated read (there is no read to gate during the walk) and why gating
only the load is **insufficient** — measured, see IMPL-AP42ZG:

> With a gate on `loadViewEntities` only, all three leak tests still FAIL. `H`
> is correctly dropped from the collections, but `V` is readable in its own
> right, so it passes both the load gate and the out-gate (`viewReader.Filter`)
> and reaches the wire.

That is the same trap RR-3T18K9 identified — result-filtering is not
source-gating — re-encountered in the refactored shape.

## The three affected surfaces

1. **`_views` API** (`api_v1.go:2609-2645`). `gateReadOrNotFound` gates the
entry, then `executeView` runs, then `buildSections(result)` → the wire
`ViewResponse` sent to the browser. The comment at `api_v1.go:2611` claims
"_views is an entity-read chokepoint just like GET /{plural}/{id}" — **it is
not**: `GET /{id}` gates the one entity; `_views` gates only the entry and
serves the traversal closure.
2. **Entity-detail side-panel sections** (`sections.go:341`). Same pattern via
a synthetic `ViewConfig`; `buildSections(result)` feeds the side panel.
3. **View commands** (`commands.go:436`). The surface whose planning found this.

## Fix

Source-gate the traversal at the two points where a hidden node can influence
the result:

1. **`traverseViewBreadthFirst`'s frontier** (`views.go`) — the load-bearing
gate, via the new `readableViewIDs` helper. Ids are resolved to types with a
content-free `store.ListEntityHeaders` scan (the gate is keyed by `(type, id)`
and the walk has no types to hand), then gated per type with one batched
`PermitsReadMany`. A denied id is not expanded, so it cannot be a
stepping-stone. This keeps the walk's "no entity loads during traversal"
property: one header scan plus one probe per distinct type per level.
2. **`loadViewEntities`** (`viewworld.go`) — batched `PermitsReadMany` per type,
so a hidden entity never enters a collection. Covers the non-recursive path,
which never enters the BFS.

Plus a defensive entry gate in `executeViewRef`. This is not redundant for every
caller: the **command surface applies no entry gate of its own**, relying on
`executeView` entirely.

All gates **fail closed** — a header-scan fault, an unresolvable type, or a
probe error drops the affected ids. Under NopACL every probe permits, so the
traversal is unchanged for a full-read principal.

**Do NOT** filter the result post-hoc — RR-3T18K9 proved result-filtering ≠
source-gating, and the load-gate-only measurement above re-confirms it.

## Scope

**In scope:**

- Source-gate the recursive frontier + the collection load, and the
`executeView` entry
- All three consumers inherit the fix
- Tests: hidden-intermediary reachability (`Entry → H → V`) for the `_views`
API and sections; where-clause-over-hidden case
- **NopACL byte-identical regression for all three consumers** — a principal
who can read the whole traversal sees no change (the guard that the fix doesn't
alter behavior for full-read principals)

**Out of scope:**

- Command payload entity/list scoping (TKT-2FDTJE)
- Making `context: view` commands grantable (TKT-2FDTJE, once this lands)

## Prevention

The systemic gap (why4/why5): the codebase's read-side ACL model is a set of
**out-filters**, and it has no vocabulary for "may not be traversed through". A
new traversal can therefore be added with no structural signal that it needs a
source gate, and — worse — a traversal that IS filtered on the way out looks
correct while still leaking reachability.

Shipped: `TestViewTraversalIsSourceGated` (`internal/dataentry/lint_test.go`), a
structural tripwire over **both** gate sites, with no skip branch (see the
`prevention` property for why that matters — the first draft of this guard would
have disarmed itself on the very next refactor).

Considered and rejected: a package-wide "no raw store reads in
`internal/dataentry`" arch-lint. It would need a large, drift-prone allowlist
(analyze tools, sync, and write-prep diffing that must stay raw per "never
redact a read that feeds a write") and would fight every change. The narrow
guard plus the CLAUDE.md convention is the honest prevention.

## Acceptance criteria

- A `_views` request whose traversal passes through a hidden intermediary does
not return descendants reachable only via that intermediary
- Entity-detail sections behave the same (hidden intermediary → its descendants
absent from the side panel)
- A `where` clause cannot be used to infer a hidden entity's property values
- Under `NopACL`, `_views` responses and section output are byte-identical to
today (regression)
- The "chokepoint" comment becomes true (corrected in `views_handler.go`, where
the handler now lives, to state that entry and traversal are gated separately)
- A prevention measure is added (see Prevention)

## Relationship to TKT-2FDTJE

TKT-2FDTJE (command payload read-gating) **depends on this**. Once `executeView`
is source-gated, TKT-2FDTJE's view-command support consumes the already-gated
traversal — no command-local traversal gating — and can lift the `context: view`
deferral safely. Entity + list scoping in TKT-2FDTJE is independent and can
proceed in parallel.
