---
id: RR-UO5XPSB
type: review-response
title: fakeGraph ignores the edge condition
finding: The test fake graph accepts a cond argument and ignores it.
severity: nit
reason: The fake serves tests of unconditional grants; conditional behaviour is pinned against the real memstore graph in grantcond_store_test.go and aclstore_test.go.
status: wont-fix
---
