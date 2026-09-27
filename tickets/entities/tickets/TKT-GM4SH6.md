---
id: TKT-GM4SH6
type: ticket
title: 'Gantt on_cycle: mark — render containment loops in place instead of blanking the chart'
kind: enhancement
priority: medium
effort: m
status: wont-fix
---

## Description

A containment loop made the whole gantt unrenderable. `on_cycle: error` is the
default and returns 422, which the SPA renders as a red error string in place of
the chart — so one bad edge between two entities blanked an entire plan.

The two existing policies were both all-or-nothing: `error` refuses the request,
`prune` drops the looping component silently. Neither can show the loop, and
neither lets the rest of the chart render alongside it.

## Approach

Added a third policy, `on_cycle: mark`. It cuts the single edge that closes the
loop and renders the component in place. The node the edge pointed at keeps its
ordinary position under its first parent and carries `in_cycle` on the wire (a
marker in the SPA); the node holding the cut edge reports `has_more_children`,
so a withheld edge never reads as a leaf. No entity is drawn twice and no
roll-up counts one twice.

**`error` remains the default**, so no existing config changes behaviour —
operators opt into `mark`.

The fold stays post-order DFS: a parent's rolled span needs every descendant's
contribution before it can be computed, so a breadth-first walk cannot produce
it. `mark` uses the back-edge information the DFS already had and was
discarding.

`reseatCycleParents` handles a pre-existing wrinkle in `multi_parent: first`:
parent selection picks a winner by sort order alone, blind to whether that
winner sits inside a loop, which could strand a node whose genuine parent was a
real root. `error` and `prune` never expose it because both discard the
component. That walk is iterative, growing reachability downward through
children — resolving parents upward would recurse on a data-controlled chain,
the goroutine-stack overflow `foldGantt` is deliberately iterative to avoid,
which kills the process rather than the request.

## Security

`in_cycle` is computed on the ACL-gated node set. A loop closing through an
entity the principal cannot see is not flagged, or the marker becomes a one-bit
oracle on hidden topology — the same reason `on_cycle: error` already evaluates
on the visible subgraph. Pinned in both directions by
`TestGantt_CycleMarkNeverRevealsHiddenTopology`.

The subtree-drill property test now runs `mark` as well as `prune`: 8 generated
graphs, every node drilled and compared byte-for-byte against the full build.
That was the likeliest divergence, since re-seating reads a candidate-parent set
holding only in-subtree edges when drilled.

## Documentation

Corrected three places (the `OnCycle` godoc and both copies of the data-entry
guide) that asserted a containment cycle is *always* a data bug. Whether a loop
is an error or a legitimate mutual containment depends on the operator's schema,
which is why `mark` exists and why its marker is worded neutrally — it reports
that an edge is not shown, not that something is broken.

## Verification

Go tests, `internal/dataentry` race-clean, frontend suite green, plus arch-lint,
plimsoll, comment-lint, golangci-lint and coverage. The existing `error` and
`prune` tests passed unchanged throughout, which is what confirms those two
policies are byte-identical.

## Resolution

Closed: Shipped in #1660. Status is wont-fix, not done, because this ticket has
no review checklist.
