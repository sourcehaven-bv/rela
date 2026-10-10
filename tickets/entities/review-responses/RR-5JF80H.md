---
id: RR-5JF80H
type: review-response
title: Dry-run coverage of thread handling was thin
finding: Only migrate_face had a dry-run thread test.
severity: minor
resolution: Added TestDropEntities_DryRunLeavesCommentThreads. rename_face, adopt-face and gc all return on !Apply before any thread call, the same early return the two tests pin.
status: addressed
---
