---
id: AM-unparseable-where-is-a-load-error
type: automated-measure
title: An unparseable view `where:` fails the load instead of widening the collection
kind: test
location: internal/dataentry/views.go (tests to be written with the fix)
status: proposed
description: "A view whose where: clause does not parse must fail at load time, never fall through to an unfiltered traversal. Pins BUG-WHEREWIDE, where a construct whose only job is to NARROW a collection failed by WIDENING it, silently — the same fail-open shape the predicate engine already refuses at load."
---

Pins BUG-WHEREWIDE.

Dropping a constraint widens the result set, so failing the load is the safe
direction — the same rule `NewEngineFromMetamodel` already applies to an
unparseable automation `when:`.

The test must assert the LOAD fails. Asserting that the filtered result is
empty would pass against a view that correctly returns nothing for an unrelated
reason.
