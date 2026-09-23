---
id: RR-S4FCL3
type: review-response
title: related() loads on unwired surfaces and fails at evaluation
finding: View condition:, next-action conditions and other surfaces compile related() and then 500 at evaluation. Refuse it at load there.
severity: minor
resolution: 'conditionlint refuses related() in view conditions, next-action conditions and form conditions (''related(...) is only supported in query_scopes''). rela validate reports it. Note: rela-server does not compile view conditions at boot for any compile error (pre-existing); it serves the ACL-scoped view.'
status: addressed
---
