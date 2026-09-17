---
id: RR-O1VNNN
type: review-response
title: Server set section truncated but the SPA never rendered it
finding: buildNestedTree correctly sets SectionData.Truncated and it reached the wire as ViewSection.truncated, but EntityDetail.vue's nested arm never read it (grep for 'truncated' in that file returned nothing). A probe with 100 parents x 24 children against the 2000-node budget emitted exactly 2000 nodes and dropped 20 parents ENTIRELY with no per-row signal, since a dropped parent has no row to carry hasMoreChildren. The list therefore read as complete while silently omitting rows -- precisely the failure the flag exists to prevent, and the gantt already renders its equivalent at GanttView.vue:410.
severity: significant
resolution: Added a truncation notice above the tree in the nested arm, rendered when section.truncated is set, wrapped in a new .nested-tree-wrap so the banner sits outside the bordered tree. Styled from existing tokens. Verified with vue-tsc. Found by probing the section-budget boundary during review rather than by a failing test -- the existing tests covered the per-parent preview cap (hasMoreChildren) but not section-budget exhaustion, which is the case with no row to attach a signal to.
status: addressed
---
