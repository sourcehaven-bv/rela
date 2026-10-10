---
id: RR-5R0DRE
type: review-response
title: No test for rename_face edges or relation writes in Tx
finding: Only migrate_face from the zero face was covered and faceMoveProbe did not observe relation writes.
severity: significant
resolution: Added TestRenameFace_CarriesTheFaceEdges; faceMoveProbe now counts CreateRelation and DeleteRelation.
status: addressed
---
