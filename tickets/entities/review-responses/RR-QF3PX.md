---
id: RR-QF3PX
type: review-response
title: Unknown relation type / target leaks raw Go error string to API client
finding: 'On unknown relation type or target, workspace.createRelation returns fmt.Errorf strings like ''target entity not found: FEAT-999'' or ''invalid relation: unknown relation: typoed''. That surfaces verbatim in the problem-details detail field, with no structure for the UI to attribute to a specific chip. Pre-validate against the metamodel and/or return a typed per-edge error.'
severity: significant
resolution: 'reconcileOutgoingRelations now pre-validates every relation type and target against the metamodel before any writes: unknown_relation_type, source_type_not_allowed, target_not_found, and target_type_not_allowed surface as *relationError with a stable Reason code, the relation type, and (for target errors) the offending target id. The handler formats this via reconcileDetail() into the problem-details `detail` so the frontend can parse it deterministically rather than scraping a Go error string. Tests assert the detail contains both the reason code and the identifier.'
status: addressed
---
