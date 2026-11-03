---
id: RR-6YIJRH1
type: review-response
title: SPA cannot resolve relation and direction to unlink
finding: The section's relation is known only via viewResult.EntryEdges for single flat follow rules from entry (views.go:48-56). Needs a server-built section descriptor; no actions without one.
severity: significant
reason: Moved with the scope split (RR-CPB5PXW) to TKT-48ZHQZ3, which carries this constraint in its description. This ticket ships no row delete, unlink or Undo action, so the defect cannot occur in this change; it must be solved before TKT-48ZHQZ3 ships.
status: deferred
---
