---
id: BUGA-I0T8L
type: bug-analysis-checklist
title: 'Analysis: PATCH entity endpoint silently drops relations payload'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

**Steps (on `/Users/jeroen/Work/VWS/clean-arch-repo`):**

1. Start `rela-server --project <path> --port 8765`.
2. Navigate to `/form/businessfunctie/PRS-BF-001`.
3. In the "Afhankelijk van externe diensten" picker, type `PRS-ED-001` and click the dropdown item.
4. Click **Save Changes**.
5. Server returns `200 OK` with `{id, type, properties, content}` (no `relations` key echoed).
6. `GET /api/v1/businessfuncties/PRS-BF-001` still returns only the original `afhankelijkVan: ["PRS-ED-002"]`.
7. `ls relations/ | grep PRS-BF-001--afhankelijkVan` lists only the old file.

**Captured PATCH payload:**

```json
{
  "properties": {"naam": "Pseudoniem maken", "status": "draft"},
  "relations": {
    "afhankelijkVan": ["PRS-ED-002", "PRS-ED-001"],
    "ondersteunt":   ["PRS-UC-001","PRS-UC-002","PRS-UC-003","PRS-UC-004","PRS-UC-005"]
  },
  "content": "..."
}
```

**Response (200):** no `relations` in the echoed entity. **Disk afterwards:**
old edge only.

Same behaviour reproduced on `applicatieflow/PRS-FLOW-001.realiseert` and every
other default-picker relation tested. The `widget: cards` path
(`applicatiefunctie.gebruikt`) works — it uses a separate `POST
/relations/{relType}` per edge.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2–3)
- [x] Systemic cause explored (why4–5)

Pinpoint: `internal/dataentry/api_v1.go:482-490`:

```go
var req struct {
    Properties map[string]interface{} `json:"properties,omitempty"`
    Content    *string                `json:"content,omitempty"`
}
```

No `Relations` field → `json.Decoder` drops the key silently. See `why1..why5`
on the bug for the chain.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

### Approach

1. Add `Relations map[string][]string` to the request DTO in `handleV1UpdateEntity`.
2. When `Relations` is non-nil, reconcile outgoing edges per relation type:
   - Snapshot current outgoing edges of the given type via `a.outgoingRelations(entityID)` filtered by `edge.Type`.
   - For each target in the new list, if not present, call `a.entityManager.CreateRelation(ctx, entityID, relType, target, RelationOptions{})`.
   - For each existing target not in the new list, call `a.entityManager.DeleteRelation(ctx, entityID, relType, target)`.
3. Only touch relation types present in the payload — omitted types are left alone (matches the frontend's current contract, which excludes `widget: cards` relations from the payload).
4. Do **not** touch incoming edges (the chip picker never mutates them) and do **not** touch relation properties (no `widget: cards` path goes through PATCH).
5. On any reconcile error, return 422 `relation_failed` with the provider error detail — the property/content update having succeeded is acceptable since the write is idempotent and the caller can retry. (Alternative: roll back. I'll keep it simple and surface the error; the per-edge endpoints behave the same.)

### Test plan

**Red tests (must fail before the fix):**

- `internal/dataentry/api_v1_test.go` — `TestV1UpdateEntity_SavesRelations`:
  - Seed ticket `TKT-001` and feature `FEAT-001` (plus `FEAT-002`) using existing metamodel fixture.
  - PATCH `/api/v1/tickets/TKT-001` with `{"relations": {"implements": ["FEAT-001"]}}`.
  - Expect 200.
  - Assert `a.outgoingRelations("TKT-001")` contains an `implements` edge to `FEAT-001`.
  - Seed a second edge to `FEAT-002`, PATCH with `{"relations": {"implements": ["FEAT-001"]}}` and assert `FEAT-002` edge was removed.
  - PATCH with `{"properties": {"title":"x"}}` (no `relations` key) and assert existing edges are untouched.

- `frontend/e2e/forms.spec.ts` — `Edit Form › saves a default-picker relation change via the API`:
  - Create a ticket + two categories via the API fixture (existing `api` helper).
  - Open `/form/edit_ticket/<id>` (existing route pattern — verified during repro).
  - Use `formPage.selectRelation('belongs-to', category2.id)` (extend FormPage if needed) or DOM-drive the `input[placeholder^="Search "]` + `.dropdown-item` click path confirmed during repro.
  - Click Save, wait for navigation.
  - Fetch the ticket via API and assert `relations['belongs-to']` contains `category2.id`.

**Green:** both tests pass after the fix. Existing `relation-cards.spec.ts` must
still pass (the cards path is untouched).

### Files to modify

- `internal/dataentry/api_v1.go` — extend `handleV1UpdateEntity`.
- `internal/dataentry/api_v1_test.go` — new test for PATCH-with-relations.
- `frontend/e2e/forms.spec.ts` — new e2e test (or a new `relations-picker.spec.ts`).

### Related areas checked

- `handleV1CreateRelation` and `handleV1DeleteRelation` (per-edge endpoints) already work correctly — used by the cards widget. No change needed there.
- The POST (create entity) handler `handleV1CreateEntity` (around line 400 area) also accepts an initial payload — I will spot-check whether it already handles relations; if it doesn't and the frontend's create flow also sends them, that's a sibling bug. Out of scope for this ticket unless red in practice; will note as follow-up if discovered.

### Risk

Low. The reconcile uses existing EntityManager methods that the cards path
already exercises. The only subtle point is idempotency — `CreateRelation` on an
existing edge should either succeed (no-op) or error cleanly. I'll verify
behaviour in the handler test.
