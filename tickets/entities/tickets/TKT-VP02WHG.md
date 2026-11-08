---
id: TKT-VP02WHG
type: ticket
title: Reconcile endpoint POST /api/v1/_reconcile
kind: enhancement
priority: medium
effort: m
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Description

Add `POST /api/v1/_reconcile` (FEAT-CMVZQFI).

- The request lists ranges. Each range names its kind (`entity` or
`relation`), the entity types and relation types in scope, a lower bound (`lo`
inclusive or `after` exclusive), an optional upper bound `hi` (absent means
unbounded), and an `op`: `fingerprint` (count and fingerprint) or `list` (the
elements, with `limit`).
- The response mirrors the list in order. A range that would list more rows
than one call covers answers `too_large`: split by type, then by address. Ranges
the deadline cuts off answer `not_run`.
- The client compares the fingerprints with its own and splits differing
ranges itself, Negentropy style. (Refined from the first draft, where the server
compared and split: the server keeps no split policy.)
- Stateless; bound to the request identity like every other read; reads
nothing the caller could not list. Never cached.
- Limits: ranges and type names per request, list rows per request, body
size, a per-principal rate limit, a slot cap per server and per principal, and a
deadline.
- OpenAPI entry and handler tests, including ACL tests (hidden rows never
appear in a fingerprint or item list).

## Acceptance

- Two principals with different read access get different fingerprints for
the same range.
- The endpoint passes the security and router tests.

## Notes

- The handler reads fingerprints through a narrow adapter over
`visibility.Elements`, so `dataentry` does not import `reconcile`. That
keeps `reconcile` out of archplan's zone of pain (Ca stays 4).
