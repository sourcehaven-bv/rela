# Data-entry API: PATCH /api/v1/{plural}/{id}

This document describes the unified PATCH endpoint for updating an entity's
properties, content, and outgoing relations atomically in a single request.

The wire format borrows the JSON:API §9 resource-identifier shape at the
relation-list level; per-edge data uses rela's existing `properties` /
`properties_unset` / `content` upsert convention. The response is rela's
flat top-level format augmented with an optional `warnings` array — we do
NOT claim full JSON:API conformance.

## Wire format

```http
PATCH /api/v1/tickets/TKT-001
Content-Type: application/json

{
  "properties": {"title": "x"},
  "content":    "...",
  "relations": {
    "tagged": {
      "data": [
        {
          "type":       "label",
          "id":         "L-001",
          "meta":       {"weight": 5},
          "meta_unset": ["added_by"],
          "content":    "edge body"
        },
        {"type": "label", "id": "L-002"}
      ]
    },
    "belongs-to": {
      "data": [{"type": "category", "id": "C-001"}]
    }
  }
}
```

All top-level fields are optional. Field absence means "leave alone".

| Field | Semantics |
|---|---|
| `properties` | Map of property names to values. **Upsert**: keys present are merged into existing properties; absent keys are unchanged. |
| `content` | Markdown body. **Upsert**: present (including empty string) replaces; absent leaves alone. |
| `relations` | Map of relation type → desired-state wrapper. See below. |

## Relations field

Each value of the `relations` map is one of TWO shapes:

### Modern (JSON:API §9-shaped)

```json
{"tagged": {"data": [{"type": "label", "id": "L-001", "meta": {"weight": 5}}]}}
```

The wrapper has exactly one field, `data`, which is an array of resource
identifiers. Three cases for `data`:

1. **Relation type absent from the map** → leave all edges of that type alone.
2. **`data: []`** → remove all edges of that type from this entity.
3. **`data: [{type, id, ...}, ...]`** → the array IS the new desired set. Edges
   in the list are kept (or upserted with new meta/content); edges currently in
   the graph but absent from the list are removed.

### ⚠️ Data-loss footgun

**Sending `data: []` deletes every edge of that relation type from this
entity.** This is the most dangerous shape in the wire format. If your client
builds the PATCH body via object spread on a not-yet-fetched form state —
`{relations: {tagged: {data: []}}}` is the default empty form value — the
first auto-save fire silently wipes the entity's tagged edges.

**Mitigations:**

- **Fetch before edit.** A client that auto-saves must complete its initial
  GET before issuing the first PATCH that touches `relations`.
- **Omit unsubmitted relations.** If the user hasn't touched the relation
  type, don't send it. Absent → leave alone is the safe default.
- **`data` field is required when the wrapper appears.** `{"tagged": {}}`
  returns 400, not a silent empty array. This catches the most common
  malformed-request case where a client constructed the wrapper but forgot
  to populate `data`.

### Per-edge fields

Each entry in `data` is a resource identifier with these fields:

| Field | Required? | Semantics |
|---|---|---|
| `type` | yes | Target entity type. **Soft check**: a mismatch surfaces a `target_type_mismatch` warning and writes the edge anyway (DEC-HWZHA). |
| `id` | yes | Target entity ID. **Soft check**: a missing target surfaces a `target_not_found` warning and writes the edge anyway. |
| `meta` | optional | Per-edge properties. **Upsert** — keys merge into existing meta. **Soft check**: unknown keys against the relation's closed schema produce `unknown_meta_key` warnings, but persist. |
| `meta_unset` | optional | Keys to clear after the merge. Mirrors `properties_unset` at the entity level. Non-string elements → 400 at unmarshal. |
| `content` | optional | Per-edge markdown body. **Upsert** — present (including empty string) replaces, absent leaves alone. **Hard 422**: present on a relation type without `content: true` (the file format can't hold a body). |

### Legacy (IDs-only)

```json
{"tagged": ["L-001", "L-002"]}
```

The legacy shape continues to work. Set semantics: edges in the list are kept
or created (with empty meta and empty content); edges absent from the list
are removed.

Mixing the two shapes in one PATCH body returns 400 with a stable
`shape_mixed` error code.

## Validation policy

Per [DEC-HWZHA](../../tickets/entities/decisions/DEC-HWZHA.md), validation
findings split into three classes:

### Hard 400 — request shape errors

Detectable without consulting the metamodel.

- Malformed JSON in body
- Relation wrapper missing required `data` field (`{"tagged": {}}`)
- Resource identifier missing `type` or `id`
- Mixed legacy + modern shapes in one body (`shape_mixed`)
- Non-string element in `meta_unset` array (`meta_unset_invalid`)
- `data` field has unexpected type (string, scalar, etc.)
- `data: null` on a wrapper (treated same as missing)
- Unknown sibling key in modern wrapper (only `data` allowed)

### Hard 422 — structural impossibilities

The storage layer literally cannot persist this state.

- Unknown relation type (`unknown_relation_type`) — no defined storage location
- Writing `content` on a relation type without `content: true` (`content_not_supported`) — the file format has no body slot

### 200 + warnings — soft conditions surfaced inline

Everything else previous drafts would 422 on. The API performs the requested
write and returns warnings in the response body so UIs surface them
non-blockingly. Each warning is `{code, path, detail}` where:

- `code` is stable and matches the corresponding `analyze_*` finding code
- `path` is an RFC 6901 JSON Pointer to the offending field
- `detail` is a human-readable explanation

Warning codes:

**Entity-level** (TKT-QETTR — soft conditions on the entity itself):

| Code | When |
|---|---|
| `required_property_unset` | Required entity property is missing or empty after the write |
| `property_type_mismatch` | Entity property value has the wrong primitive type (e.g. integer expected, string supplied) |
| `property_value_invalid` | Entity property value is the right type but outside the declared constraint (enum value not allowlisted, malformed date, bad RRULE, regex mismatch) |

**Relation-level** (TKT-6WLSW — soft conditions on a relation edge):

| Code | When |
|---|---|
| `target_not_found` | Target ID doesn't exist in the graph |
| `target_type_mismatch` | Target's actual type doesn't match `data[i].type` |
| `target_type_not_allowed` | Target type not in the relation's `to` allowlist |
| `source_type_not_allowed` | Source type not in the relation's `from` allowlist |
| `unknown_meta_key` | Meta key not declared in the relation's closed schema |
| `required_meta_unset` | Required meta property absent after merge |
| `meta_type_mismatch` | Meta value's type doesn't match the declared property type |

A read of `analyze_orphans` / `analyze_validations` will surface the same
findings; clients may de-duplicate by `code`.

Entity-level warnings reflect the **post-write entity state** — if the
saved entity has a missing required field even after this PATCH (because
the field was already missing on disk), the warning surfaces on every
write. This is by design: API contract is "what's wrong with this entity
right now," not "what this PATCH broke." UI surfaces (auto-save, etc.)
are responsible for warning-fatigue mitigation if that becomes a UX
problem.

## Atomicity

The handler runs in three phases:

1. **Validate** — relation validation runs FIRST. On 400/422, no writes occur.
2. **Update entity** — only if the request actually changed `properties` or `content`.
3. **Apply relations** — adds/updates/deletes per the diff. Soft-condition edges
   are written directly through the store, bypassing the workspace's
   target-existence check.

The validation-first ordering means a relation 422 leaves the entity
**untouched**. The unavoidable atomicity gap is mid-write-loop store failures
(disk full, permission denied, etc.) — these are rare, irrecoverable, and
return a 500 with `relation_write_failed`. The legacy reconciler had the same
property; this PR doesn't make it worse.

The store interface (internal/store/store.go) intentionally has no
transaction primitive — adding one would fight the pluggable-backends goal
(FEAT-CO4YP). For a local-first tool with single-writer-typical workloads,
the documented gap is acceptable.

## No-op suppression

The diff classifier compares each desired edge against the current graph
state. If the post-merge `(properties, content)` byte-equals the current
state, no write occurs and no SSE event fires. Auto-save's primary path
(re-saving an unchanged form) writes zero relation files and broadcasts
zero events.

Comparison uses `reflect.DeepEqual` after merging meta + applying meta_unset.
Both sides come from the same Go-native unmarshal path (JSON or YAML), so
type coercion (`int(5)` vs `float64(5)`) hasn't been an issue in practice. If
it surfaces, the fallback is a `valueEqual` helper in `internal/model`.

## Automation interaction

When `properties` or `content` changes trigger an `entity:updated` automation,
the automation runs during Phase 2. Phase 3 (relation reconcile) computes its
diff against the pre-automation graph state — **automation-created relations
that conflict with the desired set may be deleted and recreated**. This is
the same hazard the legacy reconciler has today.

## SSE events

A successful PATCH that performs writes broadcasts:

- One `entity:updated` event for the PATCHed entity, ONLY when entity properties or content actually changed
- One `relation:created` / `relation:updated` / `relation:deleted` event per actual relation write (none on a no-op)
- No events on 400/422 (handler returns before broadcast)

A relations-only PATCH that produces all no-ops emits zero events.

## ETag / If-Match

The handler honors `If-Match: <etag>` for optimistic concurrency control.
The ETag is computed against the entity's current `properties + content`.
Mismatch → 412 Precondition Failed.

## MCP and Lua content semantics

`entitymanager.RelationOptions.Content` is `*string` — pointer-vs-string
distinguishes "leave alone" from "set to empty". Two callers have notable
boundary semantics:

- **MCP `create_relation` tool**: an empty `content` string on the request
  is treated as "leave alone" (helper `nilIfEmpty` in `internal/mcp/`). To
  clear, omit `content` or pass `null`.
- **Lua `rela.update_relation`**: an explicit empty string clears the body
  (matches the user-intuition that `content = ""` means empty). To leave
  alone, omit the field.

## Action affordances (`_actions`)

Every entity and list response carries an `_actions` map describing
which write verbs the **current principal** can apply to the resource.
The Vue SPA consults this map to render write controls (buttons, menu
items); `false` verbs are hidden, `true` verbs render.

### Per-item shape

```json
{
  "id": "TKT-001",
  "type": "ticket",
  "_actions": {
    "update": true,
    "delete": false,
    "rename": true
  }
}
```

### Per-collection shape

```json
{
  "data": [...],
  "meta": {...},
  "_actions": {
    "create": true
  }
}
```

### Phase 1 verb vocabulary

The closed set of verbs in phase 1, matching `acl.Op` exactly:

| Verb | Scope | `acl.Op` |
|---|---|---|
| `create` | per-collection | `OpCreate` |
| `update` | per-item | `OpUpdate` |
| `delete` | per-item | `OpDelete` |
| `rename` | per-item | `OpRename` |

`transition:<state>` and `relation:<type>:add/remove` will follow once
the ACL layer learns to represent them (gated on a separate ACL v0.5
work item). Until then the SPA continues to render workflow controls
unconditionally and falls back to the server's 403 on disallowed
transitions.

### Always present

Every HTTP response from the data-entry server carries `_actions`.
The router unconditionally stamps a Principal on each request (a
`{User: "unknown", Tool: "data-entry"}` sentinel when no header /
environment override is configured), so there is no "anonymous"
branch in production. A principal with all verbs denied receives
`_actions: {}` — same shape, all values false. A principal with
every verb granted receives `_actions` with all values true.

### How the SPA consumes `_actions`

Phase 2 (TKT-LFT2) ships the SPA consumers. Each write affordance
(delete button, edit button, "+ New" button, drag-drop, Del-key
handler) consults `entity._actions[verb]` and renders only when the
verdict is anything other than explicit `false`. Concretely:

- `entity._actions?.delete !== false` → render the delete button.
- `entity._actions?.update !== false` → render the edit button and
  enable drag-drop in Kanban.
- `listResponse._actions?.create !== false` → render the "+ New"
  button on list / Kanban pages.
- Absent `_actions` (non-data-entry callers, pre-rollout servers) →
  defensive render; the server still 403s on the actual write.
- Direct-URL navigation to `/form/:id/:entityId` when the loaded
  entity's `_actions.update === false` → renders a "This entity is
  not editable" message in place of the form.

In **read-only mode** (`rela-server --read-only`), entity-CRUD
controls are absent across the SPA — no "+ New", no delete buttons,
no Edit buttons, drag-drop disabled. Deferred phase-2 sites (Lua
command buttons, settings / theme / git writes, relation add/remove
inside form widgets, inline-edit buttons in related-entity cards)
remain visible and 403 at the server on click; future phases gate
them as new verbs land in the ACL primitive (see TKT-XZEY).

A development-mode console warning fires once per request path when
a whitelisted API response (`listEntities`, `getEntity`,
`createEntity`, `updateEntity`) omits `_actions`. Production builds
suppress it.

### The cardinal rule

**`_actions` is a UI hint, not authorization.** The server
re-authorizes every write. A client that forges
`_actions: {delete: true}` and issues DELETE still gets the same
`403 *acl.ForbiddenError` the policy would have produced. This is
asserted as an integration test:
`internal/dataentry/affordances_contract_test.go` runs both halves
of the contract (true → 2xx, false → 403) across `NopACL`,
`ReadOnlyACL`, and `Declarative` policies. The single source of
truth for the verb→`acl.WriteRequest` translation is the
`translateVerb` function in `internal/dataentry/affordances.go`; a
grep test forbids direct `acl.WriteRequest{Op:` construction
anywhere else in the package.

### Additive evolution

New verbs are added by appending entries to `translateVerb` (and the
corresponding `acl.Op` constants in `internal/acl/`). Older SPAs
silently ignore unknown verb keys. Removing or renaming a verb is a
breaking change requiring a major API version bump.

### Scope of the invariant

The wire-vs-policy guarantee covers HTTP write endpoints reached by
the SPA. MCP write tools, Lua write paths, and scheduler-driven
writes go through the same `entitymanager.Manager` (so they are
re-authorized) but do not consult or emit `_actions`. A Lua
automation can therefore perform a write whose corresponding
`_actions[verb]` would have been `false` for the SPA principal —
that's expected and documents the scope.

## Out of scope

- **Symmetric / inverse propagation of per-edge meta.** The current write
  path doesn't propagate at all; an independent ticket would address it if
  needed.
- **Cardinality enforcement.** `min_outgoing` / `max_outgoing` are advisory
  (surfaced via `analyze_*`), never enforced at write time.
- **Granular relations diff verbs** (à la GraphQL `connect`/`disconnect`).
  v1 is replacement-only at the list level + upsert at the per-edge level.
- **Cross-entity atomic transactions.** A single PATCH targets one entity;
  there is no batch / transaction surface across entities.
