---
id: RR-H5NNGA7
type: review-response
title: Awareness ownership was not exclusive
finding: Ownership lived only in each peer's clientIDs map, so after Alice's connection removed client X another principal could claim X and keep Alice's reconnecting tab's cursor hidden.
severity: minor
resolution: ConnectionConfig.Owner binds a client id to the owner that first made it live for the room's lifetime (ownerAdmits, room.owners, capped at 4096 with non-live entries pruned first). rela passes the principal. TestAwarenessOwnership_BindingOutlivesTheConnection covers it and fails with the rule disabled.
status: addressed
---
