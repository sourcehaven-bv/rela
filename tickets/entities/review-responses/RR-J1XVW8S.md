---
id: RR-J1XVW8S
type: review-response
title: A branch reaching a final state drops siblings partway through their outputs
finding: take rejoined to a final state at once, even while a sibling still had a pending mark.
severity: minor
resolution: A branch that reaches a final state now arrives there; join ends the run in that state once no branch is pending. TestFork_FinalWaitsForPendingBranch.
status: addressed
---
