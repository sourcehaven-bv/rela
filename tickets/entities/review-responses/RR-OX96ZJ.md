---
id: RR-OX96ZJ
type: review-response
title: 'Relation body counted as hideable by visible:'
finding: 'The relation body was treated as a property that visible: could hide, but it is always served with the edge, so the audit under-reported exposures.'
severity: significant
resolution: Only relation property keys enter RelationFieldVerdicts; the body stays visible. Pinned by TestClassificationFindings_RelationVisible.
status: addressed
---
