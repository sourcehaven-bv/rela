---
id: RR-G72VZX
type: review-response
title: 'Step scanned the relation type twice and duplicated the store''s collision logic'
finding: 'Run called refuseContentScoped (a full scan) and then previewReversible (another full scan), on a path whose entire justification is avoiding per-row work. previewReversible was also a near-verbatim copy of the fallback''s pre-flight, including the error string, so the face-blind mirror bug existed in two places at once.'
severity: minor
resolution: 'Both collapsed into one call to store.CheckSwapRelationEndpoints: one scan, one implementation, one error message. The step keeps only the checks the store must not make - content-scope, overlapping endpoints and symmetry are schema questions, and a store may not read the metamodel.'
status: addressed
---
