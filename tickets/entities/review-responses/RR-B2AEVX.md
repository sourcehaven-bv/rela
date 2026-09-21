---
id: RR-B2AEVX
type: review-response
title: Reported unrelated churn was a stale local develop ref
finding: Code review N4. The reviewer flagged internal/metamodel/loader.go, internal/worlds/worlds_test.go and a docs/metamodel.md block as TKT-Z4L0IU face-primacy work riding in this branch, and recommended splitting the PR if it is meant to be per-ticket.
severity: minor
reason: 'Not a defect in this branch; no change needed. The reviewer flagged internal/metamodel/loader.go, internal/worlds/worlds_test.go and a docs/metamodel.md block as TKT-Z4L0IU work riding in this PR, and recommended splitting. Checked: those changes come from commit 0ebd7d84 (PR #1615), which is already merged upstream. `git merge-base --is-ancestor 0ebd7d84 HEAD` confirms it is in my history; against my LOCAL develop ref it appeared as new because that ref lagged origin. Diffed against origin/develop instead: 20 files, all TKT-9OFGH4, no metamodel or worlds changes. So the PR is already per-ticket. Worth recording because the reviewer''s method was right and the conclusion followed from a baseline neither of us controlled - anyone re-running `git diff develop...HEAD` on a stale checkout will see the same phantom.'
status: wont-fix
---
