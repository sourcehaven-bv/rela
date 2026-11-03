---
id: RR-8Y8I6T
type: review-response
title: Sidebar groups vanish when their sort is refused
finding: 'Server-side nav items sort on group_by and the list sort, and sidebar entities: send sort, with no unsorted fallback, so the whole sidebar group vanishes and the skip is logged only at DEBUG.'
severity: significant
resolution: 'navItemsFor reads the list again without sort when the refusal says use=sort; this covers both group_by and the list sort, since group_by only contributes to the sort. A refused filter still drops the group. useNavEntities wraps listEntities in withSortFallback. Tests: TestNavItems_RefusedSortIsDroppedRefusedFilterStands and the Sidebar.entities test for an unsorted retry.'
status: addressed
---
