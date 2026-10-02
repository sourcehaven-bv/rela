---
id: RR-3IY7G7
type: review-response
title: Label cannot shrink below 120px and overflows narrow cells
finding: 'flex: 1 0 120px gives the label a 120px floor, so alone on its line it still runs past a narrower cell into the neighbour.'
severity: significant
resolution: 'Label is flex: 0 1 120px with min-width: 0. The e2e assertion now also checks that each label stays inside its cell, at 1100px and 780px.'
status: addressed
---
