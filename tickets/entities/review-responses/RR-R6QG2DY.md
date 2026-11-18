---
id: RR-R6QG2DY
type: review-response
title: labelOf ignores a label equal to the name
finding: flowView.ts labelOf treats a label equal to the name as no label.
severity: nit
resolution: labelOf now returns the label or else the name; nothing is derived (DEC-6C1NAA).
reason: The server sends the state name as its label when none is declared, so this is the only way to tell; a deliberate label equal to the name is rare.
status: addressed
---
