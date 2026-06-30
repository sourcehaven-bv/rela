---
id: RR-LAU07P
type: review-response
title: '''has no effect; assignments empty'' warning is overbroad — the walk still feeds local-role resolution'
finding: 'warnMembershipRelationHardening warns ''has no effect'' when Assignments is empty. But globals.Members (from the membership walk) also feeds the role-relation cross-product in computeForEntity, so a configured membership relation with empty Assignments but populated role_relations is NOT inert — local roles via group members still resolve. The ''has no effect'' text is overbroad and would cry wolf on a legitimate config. Fix: tighten to ''confers no group-level roles (assignments map is empty)'' or drop the warning.'
severity: minor
resolution: Reworded the empty-assignments warning to 'configured membership_relation confers no group-level roles; assignments map is empty' and added a code comment noting the walk still feeds local-role resolution via the group-member set, so it is not fully inert. AC5 test greps for 'requires_permission'/'heeft_rol', unaffected by the wording change; still passes.
status: addressed
---
