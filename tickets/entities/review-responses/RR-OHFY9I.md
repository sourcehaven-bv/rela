---
id: RR-OHFY9I
type: review-response
title: Unclamped Policy verb method bypasses the ceiling guard and plimsoll
finding: The plan added (*Policy).RoleGrantsVerbOnFace for the unstamped fallback. It reads p.Roles[name] without the client ceiling, which TestNoDirectRoleLookupInEvaluationPaths exists to stop, and Policy is already at the 40-method plimsoll cap. Use a free function over a RoleDef plus a Request method that applies the ceiling, and an everyone-only adapter in affordances.
severity: significant
resolution: Replaced with free acl.GrantsVerbOnFace over a RoleDef plus Request.RoleGrantsVerbOnFace, which applies the ceiling and clamps through roleFor; the unstamped path answers only for everyone. No Policy method added.
status: addressed
---
