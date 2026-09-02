---
id: TKT-VR5U3Q
type: ticket
title: Decide and document the timing exposure of property-level field redaction
kind: docs
priority: medium
effort: s
status: backlog
---

## Description

Entity-level ACL visibility is pushed entirely into SQL, and the ACL-security
guide says why: *"hidden rows never leave the database ... there is no
hidden-row work to measure through timing."*

Property-level `visible:` redaction does **not** follow that rule. For pgstore
the per-field match runs in Go over the already-scanned entity, explicitly not
pushed into SQL — documented in `visiblesearch.go` as *"a tracked performance
follow-up, not a correctness gap"*. The generic bleve/linear path has the same
shape. So per-candidate work varies with whether an entity has hidden fields for
the requester: the same category of observable difference the entity-level
design deliberately avoided, framed only as performance.

GitHub issue #1094 (IB-review rela#1089, finding 2). Severity: low **because
`visible:` is not yet in production** — a reduction that expires when it ships.

## Why this is a decision, not a fix

The obvious move (SQL pushdown) is already tracked as a perf item and would
narrow the pg window without closing it — result-set size and count stay
observable wherever the matching happens — and does nothing for the default
build. Constant-time redaction means doing the hidden-field work for every
candidate on every search.

Whether a per-property timing oracle crosses the line depends on what `visible:`
is meant to defend against. Rela already treats entity *existence* as the secret
worth protecting hardest while stating that property *names* are not secret at
all, so the answer is not obvious from the existing threat model.

Options, costs and a recommendation are in
`.ignored/issue-round/DISCUSS-1094-timing-sidechannel.md`.

## Minimum outcome

Whatever is decided, the guide's entity-level *"no hidden-row work to measure
through timing"* claim needs a qualifying sentence. As written a reader
reasonably concludes the whole ACL is timing-safe, which is the actively
misleading part rather than merely the incomplete one.
