---
id: RR-NS5RNFK
type: review-response
title: Gantt edge query read every edge touching a node
finding: EntityIDs and PeerIDs with DirectionBoth mean from or to in the set, so ganttEdges read every edge touching a chart node.
severity: significant
resolution: Use DirectionOutgoing so both endpoints must be nodes; the budget test now pads edges from a chart node to children outside it and fails on DirectionBoth.
status: addressed
---
