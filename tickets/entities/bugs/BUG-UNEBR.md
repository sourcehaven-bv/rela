---
id: BUG-UNEBR
type: bug
title: PATCH entity endpoint silently drops relations payload
description: 'Editing relations via the default chip picker in the data-entry web UI appears to save successfully (200 OK, success toast) but the relations are never written to disk. Root cause: handleV1UpdateEntity in internal/dataentry/api_v1.go decodes only {Properties, Content} from the request body, so the `relations` key the frontend sends is silently dropped by json.Decoder. The `widget: cards` path uses a separate per-edge POST and is not affected.'
priority: high
effort: s
why1: PATCH /api/v1/{plural}/{id} returns 200 but the relations the frontend sent are not persisted.
why2: handleV1UpdateEntity decodes the request body into a struct that only has Properties and Content; Go's json.Decoder silently ignores the unknown `relations` key.
why3: The handler was originally built to update property/content only, assuming relations go through the per-edge POST /relations/{relType} and DELETE /relations/{relType}/{targetId} endpoints.
why4: The DynamicForm frontend was changed to pack `relations` into the PATCH body (handleSubmit in frontend/src/components/forms/DynamicForm.vue) without the backend being updated to match the new contract.
why5: 'There was no end-to-end test exercising the default chip picker''s save path against a real backend; only the `widget: cards` variant has coverage (relation-cards.spec.ts), so the drift between frontend and backend was never detected.'
prevention: Added Go handler tests and a Playwright e2e test that exercise the default chip-picker save path end-to-end — the previous coverage gap (only widget:cards was tested, via relation-cards.spec.ts) was exactly what let the frontend/backend contract drift go unnoticed. Also pre-validate relation types/targets against the metamodel in the reconcile helper so future typos or source/target-type mismatches fail fast with structured errors instead of bubbling raw Go error strings.
status: done
---

## Problem

Users report that saving relations in the data-entry edit form does not work.
Reproduction on `clean-arch-repo`:

1. Open `/form/businessfunctie/PRS-BF-001` (or any entity with the default relation picker).
2. Add a target to a relation (e.g. `PRS-ED-001` to `afhankelijkVan`).
3. Click **Save Changes**.
4. UI shows success toast and navigates; on-disk `relations/` directory has no new file; GET on the entity still returns the old edges.

Network traffic:

```
PATCH /api/v1/businessfuncties/PRS-BF-001
{"properties":{...},
 "relations":{"afhankelijkVan":["PRS-ED-002","PRS-ED-001"], ...},
 "content":"..."}
→ 200 OK
```

Filesystem afterwards still contains only the old edge — the new one is silently
dropped.

## Fix direction

Extend the PATCH request struct in `handleV1UpdateEntity` with a `Relations
map[string][]string` field and, when present, reconcile outgoing edges for each
provided relation type (add missing, remove removed) using the existing
`createRelation` / `deleteRelation` helpers. Only touch relation types present
in the payload — omitted types are left alone.

Tests:
- e2e Playwright test driving the default chip picker end-to-end.
- Go handler test that PATCHes a `relations` body and asserts the graph/disk reflects the change.
Both must be red before the fix and green after.
