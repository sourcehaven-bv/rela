---
id: RR-D2BOYL
type: review-response
title: IssuesTable rowKey can collide for two same-description rules on one entity
finding: 'IssuesTable.vue rowKey is `${checkType}:${entityType}:${entityId}:${message}`, dropping the array index the old AnalyzeView key had. Two distinct content rules with the same Description text, both violated by the same entity, produce two AnalysisIssues with identical entityId+message (message = rule.Description) -> identical Vue :key within the Validations section. Vue dev-warns and treats them as one node; the shared expandedRows entry means expanding one expands both. Old index-based key was collision-free. Fix: fold the array index back into the key (rowsFor maps with (issue, i) -> key `${i}:...`). Index is stable enough since the list only re-renders on a fresh analyze load. Add a rowsFor uniqueness test for two same-description rules.'
severity: minor
resolution: Folded the array index back into the IssuesTable rowKey (`${i}:${checkType}:${entityType}:${entityId}`) with a comment explaining why. Added AnalyzeView.test.ts case asserting two same-message rows on one entity render as two distinct, independently-expandable rows (expanding one does not expand the other).
status: addressed
---

See finding property.
