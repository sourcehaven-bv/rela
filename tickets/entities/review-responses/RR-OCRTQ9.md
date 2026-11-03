---
id: RR-OCRTQ9
type: review-response
title: Four new *Policy methods break the plimsoll method cap
finding: '*Policy already has exactly 40 methods (non-test files in internal/acl), the max-methods line, with no grandfather directive. Approach 4 (plan:298-318) adds PrincipalGrants, PrincipalReads, PrincipalFieldWrite and PrincipalRelates as *Policy methods, which makes 44, so `just plimsoll` fails CI. The same applies to realtimecfg: *validator is at 40 methods (validate.go 33, session.go 4, programs.go 2, typing.go 1), and Approach 5 (plan:326) says the validator gains a grantCheck step. Plan change: return a small value type from one acl function, e.g. `acl.JobIdentityGrants(p *Policy, user string) JobGrants`, with methods Verb(op, typ, face), FieldWrite(...) and Relates(...). Its name and godoc then limit it to job identities, which is also what you want for a ceiling-blind and local-role-blind answer that must not look authoritative for a person. Put the realtimecfg grant check on its own type in grants.go (as writeChecker does), not on *validator.'
severity: significant
resolution: 'No *Policy methods: acl.JobIdentityGrants(p, user) returns a narrow JobGrants type; the realtimecfg check lives on a new grantChecker type in grants.go, not on *validator.'
status: addressed
---
