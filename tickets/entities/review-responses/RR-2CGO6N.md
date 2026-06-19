---
id: RR-2CGO6N
type: review-response
title: No test covered the ACL create-vs-update op selection or denied path
finding: Every apply test used NopACL; nothing asserted the ACL framing actually gates an apply, and given the fail-open bug (RR-WXRLES) that's exactly where the risk lived.
severity: significant
resolution: 'Added TestApplyEntity_ACLDenied: a ReadOnlyACL principal applying is denied with *acl.ForbiddenError and nothing is persisted, proving the ACL gate is real and not bypassed. Combined with the fail-closed fix (RR-WXRLES), the create-vs-update op is now correct under transient errors AND gated.'
status: addressed
---
