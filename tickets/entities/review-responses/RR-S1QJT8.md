---
id: RR-S1QJT8
type: review-response
title: Per-surface placeholder availability is undocumented
finding: absentNote/readOnlyNote/projectionNote/notEditableNote each assemble their own vars; list and board pass only {world}, so {title} in a projection string renders literally there but substitutes on the detail page. The WorldMessages godoc implies every placeholder works everywhere. Document which placeholders each message key supports.
severity: minor
resolution: 'WorldMessages and FaceMessages godoc state per field which placeholders substitute (absent: all four; projection: {world}; stand_in: {face},{bare_face},{world}; read_only: all four) and that a placeholder a surface cannot fill is left as written; the metamodel and content-states guides say the same.'
status: addressed
---
