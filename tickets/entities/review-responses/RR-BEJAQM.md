---
id: RR-BEJAQM
type: review-response
title: Nested-section docs contradicted themselves in the same file
finding: 'Code review S1. The new prose section "Sorting is per level too" documents parent_sort/child_sort with a worked example, while thirty lines below the pre-existing bullet survived untouched: "Children render in traversal order. Section-level sorting is not supported yet." A reader who skims to the bullet list - where "what does this mode actually do" usually lives - concludes the feature does not exist. The bullet was also half-true in the worst way: wrong for nested sections, and accidentally right for flat ones because of RR-S0H0I8.'
severity: significant
resolution: 'Rewritten in the SOURCE entity (docs-project/entities/guides/GUIDE-data-entry.md) rather than the generated docs/, which is what I got wrong the first time - generate-docs.sh silently reverted an earlier edit made directly to docs/. The bullet now reads: children render in traversal order unless the section declares child_sort:, and parents likewise honour parent_sort:. Regenerated and verified the contradiction is gone from docs/data-entry.md.'
status: addressed
---
