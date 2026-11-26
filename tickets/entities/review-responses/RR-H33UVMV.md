---
id: RR-H33UVMV
type: review-response
title: Empty roles map allocated in a no-ACL build
finding: Every update of a predicate type in a no-ACL build allocates a ctx value with an empty roles map.
severity: nit
reason: One small allocation per watched write; removing it adds a branch for no measurable gain.
status: wont-fix
---
