---
id: BUG-RLBXTG5
type: bug
title: Id minting misses mixed-case ids after HighestID switch
description: Sequential id minting via store.HighestID misses hand-written mixed-case ids such as Req-5 because parsing and case handling differ per backend (RR-6JJ259C); a create can mint a colliding id.
priority: high
effort: s
why1: highestIDNumber asked HighestID for the upper- and lower-case prefix only, and fsstore, memstore and pgstore compare the prefix bytewise, so a mixed-case id such as Req-001 was not counted and the next mint could collide with it.
why2: 'TKT-FED6FMM replaced the whole-type scan (ExtractHighestNumber, which folded case) with store.HighestID, whose case handling and number parsing were never part of its contract: sqlitestore folded ASCII case and parsed digits only, the other three compared bytewise and read a leading digit run with Sscanf.'
why3: Each backend implemented HighestID on its own, and the storetest conformance suite only pinned upper-case ids, so the four backends could disagree without a failing test. The sqlite fold was pinned in a sqlite-only test whose comment said storetest could not pin it.
why4: The store interface documented HighestID by example (FEAT-042) rather than by a rule, so there was no single matching function to share.
why5: HighestID began as a fast path for one backend and grew per-backend copies; the conformance harness pins behaviour only where a test author thinks of it, so a semantic the old list-based code gave for free (case folding) was never written down as a contract to test.
prevention: entity.SequentialNumber is now the one matching rule every backend calls, the HighestID contract names it, and storetest pins case folding and digits-only parsing on all four backends (AM-highestid-case-fold-conformance).
started: "2026-11-13"
completed: "2026-11-13"
status: done
severity: major
---

## Description

TKT-FED6FMM (commit 5d3e2dd2e) switched sequential id minting to
`store.HighestID`. Its parsing and case handling differ per backend
(RR-6JJ259C), so a hand-written mixed-case id such as `Req-5` is no longer seen
when the next id is picked, which the old whole-type scan handled. A create can
then mint an id that collides with it.

## Expected

All backends agree on which ids count for the prefix, case-insensitively as
before, and a conformance test pins it (postgres is the target, but every
backend must stay correct).
