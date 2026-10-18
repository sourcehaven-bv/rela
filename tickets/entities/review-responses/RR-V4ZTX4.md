---
id: RR-V4ZTX4
type: review-response
title: Lifecycle delete of a cardinality-one channel is undone by the next post
finding: Store.Append implicitly recreates a one channel, so after delete (and its retention gate) any poster gets a fresh open channel at once. D6 covers only the never-written case. Decide whether delete of a one channel is "clear history" or should leave a tombstone that refuses posts, and add it to AC5.
severity: minor
resolution: Deleting a one channel clears its history and the next post starts a fresh channel. Documented and covered in AC5.
status: addressed
---
