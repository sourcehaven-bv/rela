---
id: RR-MUJUWS
type: review-response
title: Restriction axis lists are hand-enumerated across seven sites in two packages
finding: Narrows, denySpellings, validateSpellings, validateNoBlanks, normalized, expanded (internal/acl/ceiling.go) and unreachableTargets (internal/aclaudit/ceiling.go) each re-enumerate the same 13 Restriction fields by hand. Adding a 14th axis means touching seven sites across two packages, and the failure mode of missing one is silently inert config — exactly what A11/A13 exist to catch at runtime because it cannot be caught at compile time. A []axis{name, get, set} descriptor table iterated by all seven would make the omission a compile error.
severity: minor
reason: 'Real and correctly diagnosed, but it is a refactor of code that is currently correct and fully tested, with no behaviour change. Doing it inside the review cycle of a security feature adds churn to the diff a reviewer has already read. Worth its own ticket, and the trigger is concrete: the next axis added.'
status: deferred
---
