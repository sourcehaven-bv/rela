---
id: TKT-6WLSW
type: ticket
title: Extend PATCH /entities/{id} relations to carry per-edge meta + content
kind: enhancement
priority: medium
effort: s
status: planning
---

## Problem

The PATCH endpoint today accepts `relations: map[string][]string` — target IDs
only, no per-edge data. Auto-save (TKT-18JS6) and the `RelationCards` widget
(TKT-B9SXH) need to upsert per-edge `meta` (e.g. `weight`, `added_by`) and
`content` (markdown body for relation types with `content: true`) inside the
same single-form-PATCH so a failed save doesn't leave half the form persisted.

This ticket extends the wire format to carry that data and surfaces validation
findings as **non-blocking warnings** rather than rejecting writes — see
DEC-HWZHA "Validation policy for write APIs: tolerate temporarily invalid data".

## Wire format extension

```http
PATCH /api/v1/tickets/TKT-001
Content-Type: application/json

{
  "properties": {"title": "new"},
  "relations": {
    "tagged": {
      "data": [
        {"type": "label", "id": "L-001", "meta": {"weight": 5}, "meta_unset": ["added_by"]},
        {"type": "label", "id": "L-002"}
      ]
    }
  }
}
```

**Backwards compat**: continue accepting the existing `relations:
map[string][]string` shape (target-IDs-only) for callers that don't need
per-edge data. Both shapes follow the same validation policy; the modern shape
additionally surfaces warnings inline.

## Validation policy

Per DEC-HWZHA, validation findings split into three classes:

**Hard 400 — malformed wire format**
- Malformed JSON in body
- Relation wrapper missing required `data` field (`{"tagged": {}}`)
- Resource identifier missing `type` or `id`
- Mixed legacy + modern shapes in one body
- Non-string element in `meta_unset` array
- `data` field on a wrapper has unexpected type (string, scalar, etc.)

**Hard 422 — structural impossibilities**
- Unknown relation type (no defined storage location for the file)
- Writing `content` on a relation type whose disk format doesn't support a body (the file shape can't hold it)
- `data` value is non-array (legacy IDs-only shape with a non-array value)

**200 + warnings — soft conditions surfaced inline** Everything else previous
drafts wanted to 422 on. The API performs the requested write and returns
warnings in the response body so UIs surface them non-blockingly. Conditions:
- Target ID doesn't exist
- Target entity type doesn't match the relation's allowed `to` set
- Source entity type doesn't match the relation's allowed `from` set
- Unknown meta keys (closed-schema violation)
- Required meta property unset
- Meta value type mismatch against declared property type

Each warning is `{code, path, detail}`; `code` matches the corresponding
`analyze_*` finding code so UIs can de-duplicate against analyze runs.

## Wire format details (under the policy)

- **List replacement, edge upsert.** `data: []` removes all of that type; absent type leaves alone; `data: [...]` is the desired set. Edges in the list get their meta/content upserted; edges not in the list are removed.
- **Value-based no-op suppression.** When the post-merge state of an edge byte-equals the current state, no write occurs and no SSE event fires. Auto-save's main case (re-saving an unchanged form) writes nothing.
- **Cardinality remains advisory** (no enforcement at write time).
- **Atomicity.** Validation phase runs FIRST, before any writes (including the entity update). On any 400/422, no writes occur. Warnings do NOT block the write — the entity AND relations both persist.
- **`data` field required when wrapper appears.** `{"tagged": {}}` returns 400 (data-loss footgun mitigation).

## Approach

See PLAN-MXQKI for the implementation approach (validation-first ordering,
value-based no-op suppression, RelationOptions extension with MetaUnset and
*string Content, custom UnmarshalJSON state machine).

## Out of scope

- Frontend wiring of `RelationPicker` / `RelationCards` to the new shape — TKT-B9SXH / TKT-18JS6 will consume the API.
- Symmetric / inverse propagation of per-edge meta — current `writeRelation` does not propagate at all today (workspace.go:416). Separate ticket if needed.
- Multi-write atomicity (entity update + relation reconcile as a single transaction). The store interface (internal/store/store.go) explicitly has no transaction primitive; adding one fights FEAT-CO4YP "Pluggable store backends" goals. Documented in api-reference.md.
- Migrating callers off the legacy IDs-only shape (deprecation deferred).

## Prior art

Closed PR #648 (now-deleted branch `feat/patch-with-relations`) attempted this
on top of an older develop and ended up extracting a parallel `entitymanager`
package — but TKT-GOLNP shipped its own `EntityManager` interface in the
meantime with a different shape. This ticket is the additive subset that doesn't
fight the existing manager surface, AND aligns with DEC-HWZHA's softer
validation policy (the closed PR had the same drift toward hard 422s that this
ticket explicitly avoids).
