---
id: BUG-RILUIG8
type: bug
title: Remotes page asks update right before delete for remote_deleted entities
description: For an entity remote_deleted entry, mayWriteRemoteValue required the update right before the delete right. The guide and godoc say delete only. GitHub issue 2230.
priority: low
effort: xs
why1: mayWriteRemoteValue ran the update check for every entity entry before branching on remote_deleted.
why2: The remote_deleted branch was added inside the existing update-path function after its update guard, as an extra condition rather than a replacement.
why3: The test for the branch only varied the delete right with update always granted, so the extra requirement was never observed.
why4: The function name and godoc describe one write question, so a different verb was bolted onto it instead of judged separately.
why5: Permission tests for a gate vary only the right under test and grant the others by default, so a gate that over-asks passes.
prevention: Regression test covers delete without update. Permission tables for a gate should include a case that withholds every right except the one the action needs.
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

Taking the remote side of an entity remote_deleted entry on the Remotes page
deletes the local row on the next pull. mayWriteRemoteValue checked the update
right first, so a caller with delete but without update was refused. The
relation path, the DELETE route, the godoc and the guide all ask only the delete
right. Reported in GitHub issue 2230.
