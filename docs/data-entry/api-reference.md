# Data-Entry API Reference: PATCH /api/v1/{plural}/{id}

This document describes the unified PATCH endpoint for updating an entity,
its body content, and its outgoing relations in a single atomic request.

The wire format borrows the JSON:API §9 resource-identifier shape at the
relation-list level; per-edge data uses rela's existing
`properties` / `properties_unset` / `content` upsert convention. We do
NOT claim full JSON:API conformance — the response shape is rela's
existing flat top-level format, not the `{data: {...}}` envelope.

## Wire format

```http
PATCH /api/v1/tickets/TKT-001
Content-Type: application/json

{
  "properties": {
    "title": "New title",
    "status": "ready"
  },
  "properties_unset": ["assignee"],
  "content": "## Issue\n\n...",
  "relations": {
    "tagged": {
      "data": [
        {
          "type": "label",
          "id": "L-001",
          "meta":       { "weight": 5 },
          "meta_unset": ["added_by"],
          "content":    "edge body"
        },
        { "type": "label", "id": "L-002" }
      ]
    },
    "belongs-to": {
      "data": [{ "type": "category", "id": "C-001" }]
    }
  }
}
```

## Top-level fields

All top-level fields are optional. Field absence means "leave alone".

| Field | Semantics |
|---|---|
| `properties` | Map of property names to values. **Upsert**: keys present are merged into existing properties; absent keys are unchanged. |
| `properties_unset` | Array of property names to clear (delete the key from the entity). Keys must reference declared properties on the entity type — unknown keys → 422. |
| `content` | Markdown body. **Upsert**: present (including empty string) replaces; absent leaves alone. |
| `relations` | Map of relation type → desired-state wrapper. See below. |

## The `relations` field

Keyed by relation type. The wrapper for each relation type has exactly one
field: `data`, which is a JSON:API §9 resource-identifier array.

### Three cases for a relation type

1. **Relation type absent from the map** → leave all edges of that type
   alone.
2. **`data: []` or `data: null`** → remove all edges of that type from
   this entity. (`null` is treated as `[]`, per JSON:API §9.2.1.)
3. **`data: [{type, id, ...}, ...]`** → the array IS the new desired set.
   Edges in the list are kept (or upserted with new meta/content);
   edges currently in the graph but absent from the list are removed.

### ⚠️ Data-loss footgun

**Sending `data: []` deletes every edge of that relation type from this
entity.** This is the most dangerous shape in the wire format. If your
client builds the PATCH body via object spread on a not-yet-fetched
form state — `{ relations: { tagged: { data: [] } } }` is the default
empty form value — the first auto-save fire silently wipes the
entity's tagged edges.

**Mitigations:**

- **Fetch before edit.** A client that auto-saves must complete its
  initial GET before issuing the first PATCH that touches `relations`.
- **Omit unsubmitted relations.** If the user hasn't touched the
  relation type, don't send it. Absent → leave alone is the safe
  default.
- **`data` field is required when the wrapper appears.** `{"tagged":
  {}}` returns 400, not a silent empty array. This catches the most
  common malformed-request case where a client constructed the wrapper
  but forgot to populate `data`.

### Per-edge fields

Each entry in `data` is a resource identifier with these fields:

| Field | Required? | Semantics |
|---|---|---|
| `type` | yes | Target entity type. Must match the actual entity's type AND the relation's allowed targets — otherwise 422. |
| `id` | yes | Target entity ID. Must exist — otherwise 422. |
| `meta` | optional | Per-edge properties (typed per the relation's metamodel). **Upsert** — keys present merge into existing meta; absent leaves existing. Unknown keys → 422 (closed schema). |
| `meta_unset` | optional | Keys to clear after merge. Mirrors `properties_unset`. Unknown keys → 422. |
| `content` | optional | Per-edge markdown body. **Upsert** — present replaces, absent leaves alone. Only meaningful when the relation type is declared with `content: true`; otherwise → 422. |

## Symmetric and inverse relations

Relation types declared with `symmetric: true` or `inverse: <other-type>`
in the metamodel propagate counterparty edges automatically.

- **Symmetric**: when adding `A.T → B`, also adds `B.T → A`. Same on
  remove.
- **Inverse**: when adding `A.T → B`, also adds `B.T-inverse → A`
  (where `T-inverse` is the relation type referenced by the inverse
  declaration). Same on remove.

The inverse relation type **must exist in the metamodel** — a stale
`inverse: <ghost>` reference fails the PATCH with 422 on the first
edge that would propagate.

Per-edge meta and content are deep-copied to the back-edge — primary
and back-edge have independent property maps. Future writes to one
do not bleed into the other.

Self-loops (`A.T → A`) are NOT propagated: the back-edge would be the
same edge (symmetric) or would create a self-loop on the inverse
type (which the user didn't ask for).

A counterparty whose edges were modified via propagation receives its
own `entity:updated` SSE event. A PATCH on `A` that propagates to N
counterparties fires N+1 events total.

## Validation

Validation runs **before** the staged commit and **before** the no-op
suppression check. No request can both succeed (200) and silently drop
schema-violating data.

Validation errors fall into two HTTP status classes:

### 400 — request shape errors

Problems with the request structure detectable without the metamodel.

- Malformed JSON in the body.
- A relation wrapper missing the required `data` field
  (`{"tagged": {}}`).
- A resource identifier missing `type` or `id`.

The error response includes a JSON-pointer-style hint to the
problematic field, e.g. `/relations/"tagged"/data/0/type`.

### 422 — metamodel violations

Problems detected against the metamodel.

- Unknown relation type.
- Target ID doesn't exist.
- Target entity type doesn't match the relation's allowed `to` set.
- Meta value doesn't validate against the declared property type.
- `meta` or `meta_unset` references unknown property keys (closed
  schema).
- `content` field present on a relation type without `content: true`.
- `properties_unset` references unknown entity properties.
- The staged entity itself fails `validateEntity` (e.g., required
  property cleared).

Multiple validation errors are concatenated in the response detail —
clients see the full list, not just the first.

## Atomicity

The handler stages all writes (entity properties + content + relation
adds/removes) inside a single `WithTx` transaction. The repository's
two-phase commit applies them together:

- **Phase 1**: rename staged temp files to canonical paths.
- **Phase 2**: delete files marked for removal.

On any validation failure, no writes are staged — the transaction
rolls back without disk effect. On a Phase 1 commit failure
(filesystem error mid-rename), the in-memory graph is **never**
mutated; readers see the pre-PATCH state.

### Atomicity caveats (honest documentation)

The two-phase commit is not as strong as a database transaction:

- **Phase 1 mid-flight failure** can leave already-renamed files in a
  state that's worse than pre-PATCH for those specific files (the
  rollback deletes them, losing their prior content). The graph is
  still untouched, so the in-memory view is consistent — but the
  next reload from disk sees gaps.
- **Phase 2 failures are best-effort** and silently ignored. A
  remove-marked file that fails to delete persists on disk; on next
  reload the "removed" relation reappears.

For a local-first tool with single-writer-typical workloads, this is
acceptable. We do NOT introduce a write-ahead log in the data-entry
server.

## SSE events

A successful PATCH that performs writes broadcasts:

- One `entity:updated` event for the PATCHed entity.
- One `entity:updated` event per affected counterparty (symmetric or
  inverse propagation).

A no-op PATCH (relations identical, properties unchanged, content
unchanged) returns 200 with no events fired and no disk writes.

## ETag / If-Match

The handler honors `If-Match: <etag>` for optimistic concurrency
control. The ETag is computed against the entity's current
properties + content. Mismatch → 412 Precondition Failed.

Relation-only changes do NOT mint their own ETags; the entity's
ETag covers any combination of property/content/relation changes.

## No-op suppression

The diff classifier compares each desired edge against the current
graph state and skips writes for edges where target + meta + content
are unchanged. This is critical for auto-save: a PATCH that
re-sends the form's current state writes zero relation files and
fires zero events.

Type-aware equality: `int(5)` and `float64(5)` are treated as equal
(JSON unmarshal produces float64; YAML can produce either). Type
boundaries are respected: `int(5)` is NOT equal to `"5"`.

## Out of scope (current version)

- **Cardinality enforcement.** Relation types' min/max outgoing/incoming
  declarations are advisory (surfaced via the `analyze` MCP tool), not
  enforced at write time. This matches rela's "tolerate temporarily
  invalid data" philosophy.
- **Granular relations diff verbs.** No `connect`/`disconnect`/`set`
  operators (à la GraphQL). v1 is replacement-only at the list level
  + upsert at the per-edge level.
- **Cross-entity atomic operations.** A single PATCH targets one entity;
  propagation to counterparties is a side effect of that PATCH, not a
  multi-target transaction.
- **Full JSON:API conformance.** We borrow the resource-identifier
  shape but not the `{data: {...}}` envelope, sparse fieldsets, etc.
