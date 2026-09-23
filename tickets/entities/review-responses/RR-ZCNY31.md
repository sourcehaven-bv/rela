---
id: RR-ZCNY31
type: review-response
title: Every sidebar reload remounts every group
finding: The load generation sat in the group key so each refresh recreated all links and buttons and lost keyboard focus.
severity: nit
resolution: The group key is the index again; the load generation is part of itemKey for entities entries only. Covered by a useSidebarEmptyGroups test.
status: addressed
---
