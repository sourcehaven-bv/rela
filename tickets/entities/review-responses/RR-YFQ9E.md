---
id: RR-YFQ9E
type: review-response
title: No-fork guarantee tested only via Len==2, not directly
finding: TestRelationVersionRenameAtomicPath asserted 'no orphaned lineage' via require.Len(metas, 2), which the stitch-walk could satisfy from two lineages. The no-fork property rested only on the relations-table rel_record_id equality, not on the version rows themselves.
severity: minor
resolution: 'Added direct assertions: COUNT(DISTINCT rel_record_id) on the surviving id == 1, and total relation_versions rows == rows on rid — proving every version row sits on the one surviving lineage.'
status: addressed
---
