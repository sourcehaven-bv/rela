---
id: RR-J4B6NQH
type: review-response
title: Memory build ignores assembly failure
finding: appbuild_memory.go does not close anything when assemble fails.
severity: nit
reason: The memory build no longer serves OpenAndAssemble, and its store holds no OS resources.
status: wont-fix
---
