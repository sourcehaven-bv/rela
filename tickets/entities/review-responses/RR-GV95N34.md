---
id: RR-GV95N34
type: review-response
title: fs/mem rewrite coverage gaps are undocumented
finding: Soft-deleted bodies, bodies over MaxTargets, relation bodies and mentions added later are not rewritten. Once the old id is reused they resolve to the new entity. Docs mention only the last case.
severity: minor
resolution: GUIDE-data-entry "Mentions after a rename" lists the fs rewrite gaps (relation bodies, deleted entities, bodies over the index cap, later mentions), the audit records and successor reservation.
status: addressed
---
