---
id: TKT-7DJ2O
type: ticket
title: Migrate handlers to snapshot-prologue pattern
kind: refactor
priority: low
effort: m
status: backlog
---

## Problem\n\nThe App struct doc comment promises that handlers call `a.State()` once at entry and use the snapshot consistently. In practice, handlers throughout internal/dataentry call `a.Cfg()`, `a.Meta()`, `a.Graph()`, and `a.State()` multiple times within a single request. Each call is an independent atomic load. A reload firing between two calls produces a torn request — for example, an entity list from the old snapshot and a style map from the new one.\n\nWorst offenders:\n- handleGraphData (handlers_graph.go) — many a.Graph()/a.Meta() calls\n- handleAPIGetSettings (handlers_api.go) — interleaves State().UserDefaults / Meta() / Graph() / State().UserPalette\n- V1Config response builder (api_v1.go) — calls a.Cfg() five times and a.State() twice\n\n## Scope\n\n- Add an `s := a.State()` prologue at the top of every handler in internal/dataentry\n- Replace `a.Cfg()`, `a.Meta()`, `a.Graph()` calls within the handler body with `s.Cfg`, `s.Meta`, `s.Graph`\n- Keep the convenience accessors (`a.Cfg()` etc.) for short one-shot uses outside handlers\n- Add a strengthened concurrency test that exercises a real handler under concurrent reload and asserts cross-field invariants (e.g. entity counts in the response match the number of entities in the snapshot graph)\n\n## Acceptance\n\n1. Every HTTP handler in internal/dataentry begins with `s := a.State()` and uses `s.X` throughout.\n2. New test TestHandlerCoherenceUnderReload runs handleGraphData (or similar) concurrently with onReload and asserts each response is internally self-consistent.\n3. The App struct doc comment about the snapshot-prologue pattern is no longer aspirational.\n\n## Origin\n\nDeferred from TKT-PYN1c. See RR-Q37M1 for the cranky-review finding.
