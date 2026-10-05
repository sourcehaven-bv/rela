---
id: RR-0DVEI4
type: review-response
title: Unused role_relations and manage-roles in the fixture ACL
finding: member-of and manage-roles look copied and unused.
severity: nit
reason: 'Required: acl.yaml refuses to load a non-default world read grant while member-of is ungated (verified by removing it). Added a comment saying so.'
status: wont-fix
---

member-of and manage-roles look copied and unused.
