---
id: RR-WT1VVX
type: review-response
title: Gantt node holds one face per id
finding: nodes[id] is last-writer-wins, so once faced rows load, node.face and the ownership verdict depend on header order.
severity: significant
resolution: addGanttNodes refuses a second face of an id already in the node set (500, logged) instead of overwriting it. Pinned by TestGanttNodes_KeepTheirFace. The one-face-per-id invariant is recorded for TKT-KQXVF7 in the BUG-BZQQDP resolution.
status: addressed
---
