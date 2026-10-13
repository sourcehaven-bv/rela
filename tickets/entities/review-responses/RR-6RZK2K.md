---
id: RR-6RZK2K
type: review-response
title: Base32 path segments overflow 255-byte names for long ids
finding: Ids over ~159 chars fail Append; tombstone names fail from ~140 chars, so EntityCreated tombstone fails and the id-reuse leak opens. Cap length in Anchor.Validate or hash long ids.
severity: minor
resolution: Anchor ids capped at 100 bytes and faces at 32; longest tombstone name 244 bytes, tested.
status: addressed
---
