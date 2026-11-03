---
id: RR-4G2H6Q
type: review-response
title: Whole-message branch of readsLeftOut is untested and possibly unreachable
finding: buttons.go:291 (UsesWhole message) and leftOutRead's 'reads the whole message' text have no test. The implementer's notes say no compiling expression was found. Either find the host function that takes a record (for example a traversal or inspect-style call) and add a TestButtons_ComposeGuards case, or drop the branch and say in a comment why a record cannot be used whole. Untested refusal paths rot.
severity: minor
resolution: "addressed\tAdded TestReadsLeftOut_WholeMessage. It declares a host function that takes a record, as TestAllowRule_WholeUseWithoutTraversal does, and pins the \"message\" result and the \"reads the whole message\" text. The branch stays as a guard for a future host function."
status: addressed
---
