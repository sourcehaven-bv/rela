---
id: BUG-33Q36BI
type: bug
title: Live view misses a change reverted before the next round
description: A live view keeps a state the server reverted when the change and its revert both land between two live rounds; presence.spec.ts:60 flakes on it.
priority: medium
effort: m
why1: commitRound frames a subscription only when the round's replay hash differs from the stored baseline. A claim and release (or any change and its revert) that both land between two rounds leave the replay equal to the baseline, so no frame is sent although the client fetched the intermediate state.
why2: The baseline is the hash of the last server replay, not of what the client holds. The client's refetch after a 204 or a live:changed frame runs later than that replay and can see a newer state.
why3: liveSub assumed that any change after the baseline makes the next replay differ from it. That holds for a change that stays, not for one that is reverted before the next tick, and the 500 ms coalescing window makes that likely.
why4: The hub never observes the responses it actually serves to subscribers, only its own replays, so it has no record of an intermediate state a client saw.
why5: The live-view design modelled client state as the server's last replay instead of the responses served. No test drove a fetch between a change and its revert inside one round, so the gap only showed as a rare E2E flake.
prevention: Record the hash of each response served to a subscribed (principal, URL) pair and frame when the next replay differs from any of them. Pin it with deterministic manual-clock tests (live_served_test.go) for a lease and for a write reverted between rounds, plus no-frame tests for hidden writes and other principals' fetches.
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

## Description

A live view can keep showing a state that was reverted on the server.
`e2e/tests/presence.spec.ts:60` ("shows the claim badge on the entity, the list
and the board") hits it: it is reported `flaky` in about a third of the E2E runs
since 2026-11-02, always at line 75 (badge still shown after the release) or
line 87 (badge missing after a re-claim on the list). Each time the 15s wait
runs out, so the change is lost, not late.

## Mechanism (traced locally)

Reproduced with `--repeat-each=60 --workers=12` (2/60). Trace of a failure, in
ms since page load:

335 PUT subscriptions (entity-detail view)   -> baseline replay: no lease 341
bob claims (write lands between 341 and 429) 351 SPA refetches the view after
the 204     -> answers at 431 WITH lease 432 bob releases (done at 520) no
live:changed frame follows; the badge stays

The PUT's baseline is older than the response the client fetched after the
204. The claim and the release both land before the next round, so the
round's replay equals the baseline and `commitRound` sends no frame. The client
keeps the intermediate state.

`liveSub`'s doc says "the client's refetch after the 204 is at least as new and
any later change differs from it". The second half does not hold when the later
change is reverted before the next tick. The same window exists after every
live:changed refetch: a change after the round's replay that the client's
refetch sees, reverted before the next tick, is never framed.

## Fix direction (needs a design review)

The baseline has to be what the client may hold, not what the server last
replayed. For example, hash the responses served for subscribed (principal, URL)
pairs since the last replay, and frame when the current replay differs from any
of them. That frame only reports a difference in data the principal was served,
so it reveals no hidden change. Not done in BUG-7EDVI4C, which fixes test flakes
only.

## Fix

`liveHub.recordServed` (internal/dataentry/live_served.go) hashes each GET the
API serves to a subscribed (share key, URL) pair, below the transport gates. A
hash that differs from the baseline is kept on the subscription and puts it in
pending, so the next tick replays it. `commitRound` frames when that replay
differs from what was served before the round collected. A streak counter stops
a frame/refetch loop if a response never matches its replay.

Limit: with several server processes, a GET served by a process other than the
one holding the stream is not recorded, so behaviour there is as before.
