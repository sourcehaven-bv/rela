---
id: BUG-ULFDB
type: bug
title: '`rela graph -f svg` fails for entity types containing hyphens'
description: 'Running `rela graph -f svg` (or any non-`dot` format) fails with `<stdin>: syntax error in line N near ''-''` whenever the metamodel has an entity type whose name contains a hyphen (e.g. `review-response`, `planning-checklist`, `bug-analysis-checklist`). `rela graph` to stdout emits invalid DOT that graphviz rejects. Reproduces on the dogfood `tickets/` project.'
priority: high
effort: xs
why1: graphviz rejects the DOT emitted by `rela graph` with `syntax error near '-'`
why2: the DOT contains `subgraph cluster_review-response { ... }` — an unquoted identifier containing `-`
why3: '`generateDOT` interpolates the entity type directly into `cluster_%s`, assuming it is a valid DOT identifier'
why4: DOT unquoted identifiers are restricted to `[_A-Za-z][_A-Za-z0-9]*`; no sanitization was applied before interpolation
why5: there was no end-to-end test that rendered DOT through graphviz against a real-shaped metamodel, and unit tests only used hyphen-free type names (`requirement`, `decision`, `component`)
prevention: Sanitize any metamodel-derived string before embedding it as an unquoted DOT identifier. `sanitizeDOTID` replaces non-[A-Za-z0-9_] with `_`. Unit tests pin the rule; a shell verification script exercises the full pipeline through `dot`.
status: done
---
