---
id: RR-LXI3NW
type: review-response
title: '"Privileged role" (A2/A3) is undefined; the ACL has no built-in privilege notion'
finding: 'A2 and A3 fire on a ''privileged role'' but the ACL has no such concept (confirmed: resolver only knows grants + Permissions; holdsPermission matches exact permission strings). The plan must DEFINE privileged operationally or the checks are unimplementable / arbitrary. Proposed definition: a role is privileged if it grants any write (Create/Update/Delete non-empty or contains ''*'') OR holds any Permissions entry. Refinement: the highest-signal case is a role that holds a delegate-* permission (it can hand out access) or a wildcard write. Document this definition in aclaudit and reference it from A2/A3 so the gating is explicit and testable, not hand-wavy.'
severity: significant
resolution: Plan defines isPrivileged(role) in aclaudit = grants any write (Create/Update/Delete non-empty incl '*') OR holds any Permissions; A2/A3 reference it; delegate-*/wildcard-write is the highest-signal sub-case. Documented + unit-tested.
status: addressed
---
