---
id: RR-KDY7LE
type: review-response
title: --principal system:webhook:x --as app echoes a type the runtime ignores
finding: PrincipalType() overrides the claim, but the report printed as client type app.
severity: minor
resolution: 'checkModeFlags rejects --as and --scope for a system:webhook: principal. Test case added.'
status: addressed
---
