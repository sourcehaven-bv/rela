---
id: RR-ZXUOGC2
type: review-response
title: Parity of real GET and replay hashes is not pinned for every shape
finding: The fix assumes a real GET and a replay of the same data hash equal. TestLive_ReplayMatchesGet covers entity, list, view, world, principal_property and inbox shapes but not lease responses.
severity: significant
resolution: The lease and write tests refetch after the frame and assert no further frame. The suspect guard bounds any remaining mismatch to one frame per distinct response.
status: addressed
---
