---
id: RR-NFQ2D2G
type: review-response
title: Collapsed accordion mounts every editor; anchors land on closed cards
finding: 'RlAccordion renders the body while closed; owned redirects anchor #id onto a collapsed card.'
severity: significant
reason: Moved with the scope split (RR-CPB5PXW) to TKT-7IJ7U0Q, which carries this constraint in its description. This ticket ships no collapsible card sections, so no closed accordion mounts editors in this change; it must be solved before TKT-7IJ7U0Q ships.
status: deferred
---
