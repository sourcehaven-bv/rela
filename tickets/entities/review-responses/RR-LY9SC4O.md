---
id: RR-LY9SC4O
type: review-response
title: Delete offered on incoming owning section deletes the owner
finding: A section filled by follow_incoming of an owning relation lists the entry's owner; Delete would soft-delete the owner and cascade to the page's entity. Delete must require an outgoing owning edge from the entry (owning.go:57-62).
severity: critical
reason: Moved with the scope split (RR-CPB5PXW) to TKT-48ZHQZ3, which carries this constraint in its description. This ticket ships no row delete, unlink or Undo action, so the defect cannot occur in this change; it must be solved before TKT-48ZHQZ3 ships. Critical because delete would remove the owner; the delete action does not exist on the entity page yet, so nothing here can trigger it.
status: deferred
---
