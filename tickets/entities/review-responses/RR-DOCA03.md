---
id: RR-DOCA03
type: review-response
title: 'an unknown as= role ran the request as a privileged principal'
finding: 'buildRoleAssignee falls back to a defaultUser chosen for having UPDATE grants, so api{as="vewer"} ran as the editor and the assertion passed for entirely the wrong reason. APIResponse carried no record of the acting principal, so the failure message could not have shown it either.'
severity: critical
resolution: 'Extracted resolveRole, which reports whether the role was known; api{} refuses an unknown role naming the known ones. The empty-as default is unchanged, since it is a legitimate fallback rather than a claim.'
status: addressed
---
