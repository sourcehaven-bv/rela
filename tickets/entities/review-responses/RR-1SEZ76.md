---
id: RR-1SEZ76
type: review-response
title: Sync test depends on Ops.Sync committing before it fetches
finding: The allowed cases pass only because Sync commits before fetching from the missing remote. (cranky-code-reviewer)
severity: nit
resolution: Kept as is.
reason: If Sync is reordered the test fails falsely, never passes falsely, so it cannot hide a broken gate. A bare origin repository adds setup without adding coverage of the gate.
status: wont-fix
---
