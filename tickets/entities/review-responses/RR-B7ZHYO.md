---
id: RR-B7ZHYO
type: review-response
title: analyze_* tools aggregate over the whole graph; schema helpers take wide store.Store and cannot accept a gated reader
finding: analyze_unique (tools_analysis.go:171) leaks entity ids AND property values; analyze_cardinality (:108), analyze_properties (:216,:239) and analyze_schema (:337) enumerate or count the whole graph. schema.ValidateRelationProperties and schema.StoreCounter take the wide store.Store, so they cannot accept a narrowed gated read interface without change.
severity: significant
status: open
---

## Finding

The `analyze_*` tools operate over the **entire graph** by design, so injecting
a gated reader into the entity tools does not make them safe:

- `tools_analysis.go:171` — `analyze_unique` reports same-type entities sharing
a property value. This leaks entity ids **and property values**.
- `tools_analysis.go:108` — `analyze_cardinality` enumerates violations.
- `tools_analysis.go:216`, `:239` — `analyze_properties`, incl.
`schema.ValidateRelationProperties(ctx, s.deps.Store, s.deps.Meta)`.
- `tools_analysis.go:337` — `analyze_schema` via `schema.StoreCounter{Store: ...}`.

`analyze_orphans` is NOT affected provided the tracer is swapped:
`visibility.VisibleTracer` implements `FindOrphans`
(`internal/visibility/tracer.go:192`), `FindPath` (`:141`) and `HasCycle`
(`:221`).

**Complication:** `schema.ValidateRelationProperties(ctx, st store.Store, ...)`
(`internal/schema/validate_properties.go:54`) and `schema.StoreCounter{Store
store.Store}` (`internal/schema/store_adapter.go:10`) depend on the **wide**
`store.Store` composite, so they cannot take a narrowed read interface without
signature changes in `internal/schema`.

## Resolution options

- **(a) Recommended for this ticket:** exclude the whole-graph analyzers
(`analyze_unique`, `analyze_cardinality`, `analyze_properties`,
`analyze_schema`) from the *remote* tool set, as with Lua. Keep them on stdio.
Cheap, and consistent with the ticket's "smallest correct remote surface".
- **(b)** Adapt `internal/schema` to a gated reader and define
`filtered_count` semantics per DEC-RG878 — a distinct feature deserving its own
ticket.

Whichever is chosen must be pinned by a test (extend AC #7).
