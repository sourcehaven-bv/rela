---
id: RR-PABTYTY
type: review-response
title: Client-side relink as Undo destroys hidden relation data
finding: deleteRelation is a hard delete; hidden relation properties and body are lost, history forks, create automations re-fire, create grant may be missing. Unlink needs a confirm without Undo, or a server-side reversible unlink.
severity: significant
reason: Moved with the scope split (RR-CPB5PXW) to TKT-48ZHQZ3, which carries this constraint in its description. This ticket ships no row delete, unlink or Undo action, so the defect cannot occur in this change; it must be solved before TKT-48ZHQZ3 ships.
status: deferred
---
