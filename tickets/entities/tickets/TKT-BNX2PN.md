---
id: TKT-BNX2PN
type: ticket
title: Gate _views read path through the ACL read gate (TKT-VQGN follow-through)
kind: refactor
priority: high
effort: s
status: done
---

handleV1Views (GET /api/v1/_views/{type}/{id}) serves the full entity —
`_title`, properties, and **content body** — via executeView +
serializeEntityForWire with NO read-gate check, so any authenticated principal
reads any entity by id regardless of ACL. This is read-side coverage TKT-VQGN
established for handleV1GetEntity (gateReadOrNotFound) but never extended to the
auxiliary `_views` endpoint.

Confirmed in a live pen-test: a zero-grant principal read TKT-1's title and
content body via `_views` while GET /tickets/TKT-1 correctly 404'd.

**Fix:** call `gateReadOrNotFound(w, r, entityType, entityID)` after the
entity-type check, before executeView. Add a read-gate test asserting a hidden
id returns 404. Highest severity of the four findings (full content disclosure).
