---
id: TKT-6N9O1Y
type: ticket
title: Gate _sidepanel read path through the ACL read gate (TKT-VQGN follow-through)
kind: refactor
priority: medium
effort: s
status: done
---

handleV1SidePanel (GET /api/v1/_sidepanel/{form}/{id}) calls `a.getEntity` +
executeSidePanel with no read-gate check, so a principal who cannot read the
entry entity can still trigger the side-panel traversal and receive the entry
plus related entities. Requires a configured `form.side_panel` to exploit, but
the code path is structurally unguarded — same class as `_views` and the
original TKT-VQGN gap.

**Fix:** call `gateReadOrNotFound(w, r, form.EntityType, entityID)` before
getEntity/executeSidePanel. Add a read-gate test.
