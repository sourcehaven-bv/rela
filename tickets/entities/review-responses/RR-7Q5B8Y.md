---
id: RR-7Q5B8Y
type: review-response
title: PrincipalGrants as specified cannot express read, misses `everyone`, and the guard-test claim is wrong
finding: 'acl.Op has no read verb (internal/acl/acl.go:93-97), and grantsVerb (policy.go:568) handles only create, convert, update and delete. So `PrincipalGrants(user, op acl.Op, target)` cannot answer the plan''s "read on each attach_to type". Read grants live in RoleDef.Read and Worlds. The generalized principalGrantsCreate (policy.go:1640) also ignores the built-in `everyone` role (EveryoneRole, policy.go:38), so a run_as that gets update only through everyone fails to load. A run_as whose only grant is a local role fails to load as well. The plan calls the check a "floor", but a miss is a load error, so it is a hard gate. Finally, ceilingguard_test.go exempts policy.go as a whole file, and its regexp only matches `policy\.Roles\[` (ceilingguard_test.go:21-34). There is no per-function entry to join. Plan change: give read its own helper over RoleDef.Read (or a separate verb type), count the everyone role, document that a local-role-only grant for run_as is refused at load, and drop the ceilingguard edit.'
severity: minor
resolution: 'Deferred to 5b: a read helper over RoleDef.Read, counting the everyone role, refusing local-role-only run_as grants, and no ceilingguard edit are listed under Deferred to 5b.'
status: addressed
---
