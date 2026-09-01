---
id: RR-Y9C2DP
type: review-response
title: writeEntity/writeRelation are now pure one-line pass-throughs — 2 free methods for a later arc step
finding: After the mdCodec extraction, FSStore.writeEntity (entity.go:499) and FSStore.writeRelation (relation.go:242) are one-line pass-throughs to s.codec.*. Inlining their six in-package call sites would remove 2 more methods from the count. Note these were ALREADY pass-throughs before this PR — not a regression introduced here.
severity: nit
reason: 'Deliberately deferred: inlining would add non-mechanical churn to a PR whose entire value is being mechanically verifiable against the storetest conformance harness. Reviewer explicitly agreed this was the right call for this PR. Fold into whichever later arc step next touches entity.go/relation.go (a pure sed across six in-package call sites) rather than spending a PR on it.'
status: deferred
---

Nit from the TKT-Y683LJ code review (cranky-code-reviewer, PR #1465), logged
against the TKT-N0IKN9 arc as a cheap follow-on win.

Process note from the same review, worth carrying into later arc steps: verify
in the worktree that actually has the commit checked out and use `go test
-count=1`. The reviewer's first run silently tested `develop` because `git
checkout` failed (branch already checked out elsewhere) and the cached PASS
looked identical to a real one.
