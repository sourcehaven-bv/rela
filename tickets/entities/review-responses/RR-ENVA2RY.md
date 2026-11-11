---
id: RR-ENVA2RY
type: review-response
title: markSynced records a sync while the sides still differ
finding: markSynced sets Synced on a row whose fields are held; a later latest decision then sees both times before the sync and falls back to hold.
severity: minor
reason: Falling back to hold is the safe direction and matches the entity path from step 9a. Changing what Synced means touches every rule; tracked for step 10 with the status page.
status: deferred
---
