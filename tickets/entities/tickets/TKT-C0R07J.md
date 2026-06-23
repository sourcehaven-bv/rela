---
id: TKT-C0R07J
type: ticket
title: Gate _documents read path through the ACL read gate (TKT-VQGN follow-through)
kind: refactor
priority: medium
effort: s
status: done
---

handleV1Documents (GET /api/v1/_documents/{name}/{id}) fetches the entity
(GetEntity) and runs the document renderer (command or Lua script) with no
read-gate check. After the entity_type guard it renders and returns document
HTML plus EntityIDs for any authenticated principal, regardless of read ACL. A
Lua document script can additionally read related entities, widening the
disclosure.

**Fix:** call `gateReadOrNotFound(w, r, docCfg.EntityType, entityID)` after the
entity_type match and **before** rendering — a denied principal must not trigger
the (possibly Lua) renderer at all (no user Lua on the read path for a denied
caller). Add a read-gate test.
