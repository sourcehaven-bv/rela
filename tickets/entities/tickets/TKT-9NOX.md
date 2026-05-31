---
id: TKT-9NOX
type: ticket
title: Thread caller context into affordance predicate Eval
kind: refactor
priority: low
effort: xs
status: done
---

## Goal

`affordances.PolicyResolver.passes` called `prog.Eval(context.Background(), b)`,
dropping the caller's request context before predicate evaluation and the
host-function calls (`has_relation`, `count_relations`, local-role `has_role`)
it makes. This is inconsistent with the caller-ctx pattern established in
TKT-WFB6 / PR#825, where read bindings thread the caller context so
cancellation, deadlines, and request-scoped values propagate.

Surfaced as a low-severity IB-review finding (CISO / tschmits) on PR#841,
flagged as mechanical-to-fix before GA. No functional bug today (the lookup is
in-memory), but it's a latent correctness gap the moment a host func does
ctx-aware work (cancellation, tracing).

## Fix

- `bindingContext` gains a `ctx` field, set from the caller's context
in `bindingFor`.
- `passes` evaluates with `prog.Eval(bc.ctx, b)` instead of
`context.Background()`. Because `predicate.Program.Eval` threads its ctx into
every host-function `Call`, this fixes both the Eval and the host-func paths in
one change.

## Test

`TestResolver_CallerContext_ThreadsToHostFuncs`: a predicate invoking
`has_relation` is evaluated with a marker value on the context; a ctx-recording
`RelationLookup` asserts the marker reaches `OutgoingCounts` — proving the
caller context is threaded, not `context.Background()`.

## Out of scope

The other PR#841 IB findings (the `default`->`everyone` startup warning, and
`handleV1CreateEntity` not validating field-affordances) are tracked separately
— both are pre-GA hardening, not this mechanical ctx fix.
