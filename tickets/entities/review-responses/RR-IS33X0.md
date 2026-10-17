---
id: RR-IS33X0
type: review-response
title: What the mapping needs was incomplete
finding: 'Missing: transition-guard permissions for status sets, read at the find face/world, and HoldsPermission checks global roles only.'
severity: significant
resolution: Read at find.face is checked. Transition-guard permissions and the worlds axis are not checked; docs say so. Permissions use HoldsPermission (global roles), which is exact for a hook principal with no user entity; documented in the code.
status: addressed
---
