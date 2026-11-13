---
id: RR-GRODXA3
type: review-response
title: '[security] Declared principal groups could overwrite acl.yaml assignments'
finding: A declared principal's generated group could collide with an acl.yaml assignment and replace its roles.
severity: minor
resolution: assignRoles checks collisions with acl.yaml and group assignments and fails the case. TestPrincipalGroupCollision covers it.
status: addressed
---
