---
id: FEAT-79DTF9
type: feature
title: Declarative next-action layer
summary: Operator-configured rules that derive one suggested follow-up from graph state and surface it as an advisory hint in the UI.
description: Operator-configured sources derive a suggested follow-up from graph state and surface one at a time in the UI — advisory ("could do"), never a task queue. Sources yield entity-shaped candidates; operator-defined ordered bands rank them, with stable-random selection within a band. Snooze/mute/cooldown live in a per-user state service (mem/disk/postgres), never in the graph, and resolved suggestions are never cached because a cross-principal cache would defeat the ACL gate. Phase 0 ships without store changes; Phase 1 extends store.GraphQuery with property predicates.
priority: medium
status: proposed
---

## What

Operator-configured sources that derive a **suggested** follow-up from graph
state and surface **one at a time** in the UI. An advisory hint — "this proposal
has sat unanswered for eleven days, chase it?" — not a task queue.

Every system built on rela reimplements some version of this, badly and bespoke.
It sits naturally on the state machines rela already has: `transitions:` tells
you the stages; the gap is expressing what follows from being stuck in one.

## Framing constraints (load-bearing)

- **Advisory, not a task queue.** A hint, not a demand.
- **Things a user *could* do, not *should* do.** Distinguishes this from
analysis/validation output, which has an opinion about correctness.
- **One suggestion at a time.** A helpful companion, not an overloaded todo list.
- **Good, not optimal.** Picking one of several good next actions is the goal;
avoiding a bad one is the bar.

The one-at-a-time constraint is what makes bounded candidate sets non-lossy,
makes ranking explicability cheap, and removes any "show me all 12" surface to
grow into.

## Design

See [[RES-09YLLL]] for the full survey: the model, twelve decisions with
rationale, eight grounding scenarios, and the phased build order.

Key shape:

- **Sources** yield 0..n entity-shaped candidates; the engine shows one.
Independent and separately deletable — that is the unit of iteration.
- **Bands** are operator-defined and ordered; stable-random within a band
(dwell-time ordering rejected).
- **Suggestion key** `(source_id, entity_id, optional entity.prop(s))` drives
cooldown, snooze and dedup.
- **Per-user state** (snooze / mute / cooldown) lives in a new service with
mem / disk / postgres backends — never in the graph.
- **No caching**: a cache across principals is a footgun against the ACL gate.

## Scope

Phase 0 needs **no store changes**. Phases 1–2 and the deferred query-layer
extensions are enumerated in the research.
