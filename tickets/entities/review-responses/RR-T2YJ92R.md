---
id: RR-T2YJ92R
type: review-response
title: rela validate continues with an unexpanded acl policy
finding: 'The ValidateACLPolicy error was ignored, so visible: checks on abstract types would miss and fail open.'
severity: minor
resolution: rela validate skips the flows check with a warning when acl.yaml does not validate; LoadFlows godoc states the precondition.
status: addressed
---
