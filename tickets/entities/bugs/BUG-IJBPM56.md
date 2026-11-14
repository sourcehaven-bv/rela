---
id: BUG-IJBPM56
type: bug
title: Rejected channel batches are not charged against the rate limit
description: PostBatch charged the rate limit only after every message passed validation, so an invalid batch never got a 429.
priority: low
effort: s
why1: PostBatch called chargeBatch only after checkLogProps passed for every message, so an invalid batch returned 400 without spending a token.
why2: The order was chosen on purpose in review (RR-8DNAYO7, RR-FAW4JGH) so that a refused batch costs nothing, which suits an honest client fixing its input.
why3: The review weighed only the honest client; the abuse case, a poster replaying an invalid batch to force validation and ref resolution, was not considered.
why4: The single-post path documents where it charges (an invalid message is charged), but nothing required a new post path to charge at the same point.
why5: There was no shared rule or test that every channel write path charges the rate limit before the expensive validation work.
prevention: TestPostBatch_Charges pins that an invalid batch spends its tokens and gets a 429; the PostBatch godoc and the realtime guide state that a batch is charged at the same point as a single post.
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Summary

`realtime.Service.PostBatch` validated every message (`checkLogProps`) before it
charged the rate limit (`chargeBatch`). A single post charges before the
properties are checked (`checkLogPost`). A batch with an invalid message was
therefore never charged and never got a 429, even a batch of one. This applied
to `POST …/_batch`, MCP `post_messages` and `rela channel post-batch`.

An authenticated poster, such as an agent in a retry loop, could force unbounded
validation and ref resolution (up to 10 messages of 16 KiB per request).
Availability only; no data disclosure and no bypass of the post limit for
accepted messages.

Reported in GitHub issue #2236 (security review of #2210).

## Fix

`PostBatch` now calls `chargeBatch` right after the session generation check,
the same point where `Post` charges. A batch refused before that point (denied,
unknown type, signal, closed channel, wrong session state, stale generation)
still costs nothing. A batch with an invalid message spends its tokens, like an
invalid single post. `TestPostBatch_Charges` pins this.
