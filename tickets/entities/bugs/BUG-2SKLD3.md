---
id: BUG-2SKLD3
type: bug
title: pgstore filters faces before the world picks the prime
description: Under a non-default world pgstore filtered faces on Props/Narrowing before the world's DISTINCT ON rank, so a lower-ranked face answered for an entity whose prime failed the filter.
priority: high
effort: s
why1: pgstore's graph query puts Props and Narrowing in the WHERE clause that DISTINCT ON ranks over, so the filter runs before the world picks the prime.
why2: The world scope (TKT-WAV8XP PR-C) was added by reusing the existing single-SELECT shape, which was correct for the default world where each id has one row.
why3: No conformance case combined a world with property predicates; the storetest Worlds suite exercised only unfiltered world queries.
why4: Each backend is checked against graphquerynaive one feature suite at a time, so interactions between GraphQuery fields (world × props) are covered only when someone writes that case by hand.
why5: There is no differential harness that compares a SQL backend with the naive reference over generated combinations of GraphQuery fields.
prevention: Conformance case Worlds/PropsFilterThePrimeNotALowerRankedFace on every backend, and the storetest graph differential harness (TKT-B51CYD) run on postgres and sqlite with worlds in its generator.
status: done
---

## Description

Under a non-default world, pgstore renders a graph query's `Props` and
`Narrowing` in the same WHERE clause as the world's `DISTINCT ON` ranking. It
therefore ranks only the faces that already pass the property filter. When the
world's prime fails the filter, a lower-ranked face that passes is returned
instead.

graphquerynaive, and the documented semantics ("a world picks the face, a scope
decides whether the row is in the list"), resolve the prime first and then
filter it.

Repro: `page` PF-1 with default face `status=open` and `published` face
`status=closed`; world `[published]`, fallback default. `GraphQuery` with
`status == open` returns PF-1 (its default face) on postgres and nothing on
memstore/fsstore. `GraphCount` and `CountMatched` agree with the rows, so the
list total is wrong too.

Pinned by `storetest` Worlds/PropsFilterThePrimeNotALowerRankedFace.

Found during the TKT-B51CYD design review.
