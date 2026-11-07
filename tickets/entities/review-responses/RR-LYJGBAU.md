---
id: RR-LYJGBAU
type: review-response
title: Wildcard guard regex misses some spellings
finding: 'wildcardguard_test does not catch case "*": or "*" == x.'
severity: nit
reason: No such spelling exists in internal/acl; the guard is a tripwire, not a parser.
status: wont-fix
---
