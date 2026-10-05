---
id: RR-8WU7X7
type: review-response
title: 'PR 8: no archguard rule for GraphQuery literals'
finding: A zero GraphQuery selection fails at run time as a 500; nothing stops a new literal without Faces.
severity: minor
resolution: Added TestNoNewUnselectedGraphQueries with a shrink-only allowlist naming the ACL template, the scope-lowering fragments and the stampScope completions.
status: addressed
---
