---
id: RR-HSAKHS
type: review-response
title: Member reachability loop ran one pass
finding: The grew loop in RoleRelationsNoMemberHolds stopped after one pass.
severity: nit
resolution: It was correct but read as a loop; replaced by a memberTypes helper with one check and a comment saying why one check suffices.
status: addressed
---
