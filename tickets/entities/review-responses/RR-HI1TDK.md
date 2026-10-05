---
id: RR-HI1TDK
type: review-response
title: 'A16 scope: which role-conferral endpoints must be faceless'
finding: A16 could be read as requiring every endpoint of every ACL relation to be faceless. The implementation refuses faced users, members, groups and role holders, and content scope on walked relations, but allows faced role targets, faced inherit_roles_through endpoints, role_relations without confers, and undeclared relations.
severity: minor
reason: 'Deliberate interpretation, reviewed by cranky-code-reviewer and rela-security-reviewer as security-neutral: a conferred role applies to the whole target entity and face-named write grants decide the faces; containment inherits by entity id; a relation without confers is never walked; an undeclared relation holds no edges. Recorded in the validateIdentityStructure godoc and GUIDE-acl-security.'
status: wont-fix
---
