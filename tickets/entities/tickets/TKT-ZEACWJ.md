---
id: TKT-ZEACWJ
type: ticket
title: Array indexing and per-element iteration for webhook payloads (Alertmanager, Grafana)
kind: enhancement
priority: high
effort: l
tags:
    - needs-design
status: ready
---

Declarative webhook interpolation ([[TKT-1EM4KL]]) cannot index into JSON
arrays, so the two most common monitoring webhook formats cannot be ingested at
all without preprocessing.

Measured through the production router:

| Template | Result |
| --- | --- |
| `{{body.alerts.0.labels.alertname}}` (Alertmanager v4) | `""` — **silent miss** |
| `{{body.evalMatches.0.metric}}` (Grafana legacy) | `""` — **silent miss** |
| `{{body.commonLabels.severity}}` | `"critical"` — nested objects work |
| `{{body.status}}` | `"firing"` — top-level scalars work |
| `{{body.n}}` where n = 1756713600123456789 | `"1756713600123456768"` — precision lost |

## Who this blocks

- **Prometheus Alertmanager** — every alert's `labels`, `annotations`,
`fingerprint` and `startsAt` live inside `alerts[]`. Blocked entirely.
- **Grafana 9+ unified alerting** — Alertmanager-compatible payload. Blocked.
- **Grafana ≤8 legacy** — `evalMatches[]`. Blocked.
- **Icinga 2** — unaffected. The operator hand-writes the body in a
NotificationCommand and would naturally emit a flat object.

## Why it is worse than a missing feature

An unresolved reference becomes the empty string **by design** (a stored
`{{body.host}}` would be a silent corruption that looks like a template bug
forever). So an operator wiring up Alertmanager gets entities created with empty
titles rather than an error, and the misconfiguration is visible only by
inspecting stored data.

**Batching compounds it.** Alertmanager groups multiple alerts into one POST,
and the pipeline creates or updates at most ONE entity per delivery. Even with
array indexing, `alerts[1..n]` would be silently dropped — so per-element
iteration is part of the same problem, not a follow-up.

## The right vehicle is `internal/predicate`, not the `{{...}}` substituter

`{{...}}` is substitution-only with no call syntax, so bolting array access onto
it means inventing a second expression dialect. rela already has one:
`internal/predicate`, the sandboxed Lua-expression subset used by ACL
affordances, state-machine transitions, automation conditions and validation
rules.

It already has most of what is needed:

- `ListType` and `NewList` (`internal/predicate/value.go:140`).
- A distinct `Int` type alongside `Number` — which would also fix the float64
precision loss above.
- `predicatefns` already declares list-typed record fields
(`predicatefns/env.go:94`).

What it does **not** have: numeric indexing. `walkAttrGet`
(`internal/predicate/walk.go:24-49`) requires a **string literal** key and a
`RecordType` object, and explicitly rejects a computed key. So `alerts[1]` is
refused by the grammar today and needs a new walk case plus an IR node.

## Scope

- Numeric index access on `ListType` in `predicate` — grammar, IR, evaluator,
and the SQL-portability metadata (an index is presumably not portable; say so
rather than leaving it implied).
- A per-element iteration step in the webhook vocabulary, so one delivery can
create or update N entities. This is where the design work is: what identity
each element gets, what the response reports when 3 of 5 elements conflict, and
whether partial success is expressible.
- Decide whether an unresolvable path stays a silent empty string or becomes an
error. Silence is right for an optional field and wrong for a typo'd one; the
current rule cannot tell them apart.

## Design questions

- **Blast radius.** `predicate` is shared by five surfaces. A grammar addition
is not webhook-local, and an accepted IR node must keep identical semantics
across every evaluator and future SQL target (the package doc is explicit about
this). Needs its own review.
- **Iteration and the retry budget.** [[TKT-1EM4KL]]'s executor retries a whole
delivery on conflict. If a delivery is now N writes, a retry must not re-apply
the ones that already succeeded.
- **Precision.** Whether to decode JSON numbers via `json.Number` so large
integers survive, and what that means for existing `{{body.n}}` users.

## Out of scope

- Full JSONPath. The goal is reaching an element and iterating, not a query
language.
