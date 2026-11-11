---
id: RR-RV6KATO
type: review-response
title: fork() does not check fk.State and reports a vague error past MaxBranches
finding: fork never checks fk.State == cur.State, and more than MaxBranches targets reports no declared branches.
severity: nit
reason: flowcfg caps targets at 8 (= MaxBranches) and keys forks by their state, so neither can happen.
status: wont-fix
---
