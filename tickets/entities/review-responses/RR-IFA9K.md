---
id: RR-IFA9K
type: review-response
title: Server isSafeReturnPath case-asymmetric vs client
finding: internal/dataentry/return_path.go:29 checks /%5C and /%2F uppercase only; frontend/src/utils/returnPath.ts:67-74 checks both cases. An attacker-supplied ?return_to=/%5cevil.com (lowercase) passes the server guard, the rewriter appends it verbatim into rendered HTML. Client rejects on render, but the string now sits in URL bar, history, logs. Any future consumer trusting the server guard is vulnerable. The ticket widens server guard's blast radius from form-only to every screen; asymmetry becomes exploitable by accident. Must fix in this ticket, not deferred.
severity: critical
resolution: 'Brought server isSafeReturnPath fix into scope (scope item #2). AC8 added: prefix checks on /%5c/%5C/%2f/%2F become case-folded. Table test extends return_path_test.go with the lowercase cases.'
status: addressed
---
