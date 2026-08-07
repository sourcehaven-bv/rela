---
id: TKT-OGR566
type: ticket
title: Bound concurrent Lua document renders with a shared pool
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

There is no concurrency limit anywhere on the Lua document render path —
verified by grep across `internal/dataentry`, `internal/script` and
`internal/lua`. Every render is a goroutine that runs arbitrary user Lua over
the graph until it finishes or hits its per-document `timeout:`.

`internal/cmdexec` already solves the same problem for external commands, and
this repo's own `CLAUDE.md` states the rule:

> a bounded pool capping concurrent runs [...] The transform engine must be
> built ONCE and shared, not per request — it owns the bounded pool, so a
> per-request engine gives every request its own pool and the concurrency cap
> bounds nothing.

Lua renders never got the equivalent.

## Why it matters more since TKT-M1AX6P

Standalone documents raise the likelihood (not the ceiling):

- they aggregate across the whole graph by design;
- they sit on a **sidebar link**, so every user opens them, rather than only
users already on one entity's page;
- `DocumentView.vue` re-renders on every `entity:changed` SSE event, which is
type-scoped with no id filter.

So N users with a report open plus a bulk import gives N concurrent full-graph
aggregations per write event. Singleflight does not help: it only ever collapsed
identical `(principal, entry, config)` triples, and deliberately does not apply
to standalone renders (a key that collapses two principals is the RR-2QSGLU
cross-principal hazard).

## Proposal

A bounded pool over the Lua render path, built once and shared, following the
`internal/cmdexec` pattern. Applies to all three render entry points
(`ExecuteDocument`, `ExecuteListDocument`, `ExecuteStandaloneDocument`), not
just the standalone one — the exposure is shared.

Decide the over-capacity behaviour explicitly: queue with a deadline (risking
pile-up) vs. shed with a 503. Consider also debouncing the SSE-triggered
re-render client-side, but treat that as a mitigation, not the bound — a
malicious or buggy client ignores it.

## Origin

RR-P4E9GL, from the code review of TKT-M1AX6P.
