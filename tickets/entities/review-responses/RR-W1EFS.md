---
id: RR-W1EFS
type: review-response
title: Self-referential rename writes RelationsUpdated as 2 (cosmetic, pre-existing)
finding: For a self-referential relation A--rel-->A, buildRenameResult includes the relation in BOTH the outgoing and incoming RelationsUpdated lists (count == 2). The CLI displays this to users as if it were two relations. The current test asserts the incorrect behavior. Pre-existing in the old rename.go; this PR preserves it for parity. Worth fixing in a follow-up that updates both buildRenameResult and the test assertion.
severity: minor
reason: Pre-existing CLI cosmetic; preserving parity in this PR avoids a behavior change that would surprise users. Tracked as a follow-up cleanup.
status: deferred
---
