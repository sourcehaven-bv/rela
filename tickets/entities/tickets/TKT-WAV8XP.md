---
id: TKT-WAV8XP
type: ticket
title: 'Worlds: metamodel declaration, resolver, pushdown, selection (Step 2)'
kind: enhancement
priority: high
effort: xl
status: done
---

Design doc §4. Worlds as metamodel objects: `select` (with fallback chains),
per-type `overrides`, mandatory `otherwise: exclude|default` validated at load.
Default world total by construction — a metamodel with no `worlds:` block
behaves byte-identically to today.

- Resolver as a read decorator above the store (visibility/DEC-ZBI39P pattern); tracer/analysis/export run over a resolved world unchanged.
- **World required in the interior** — no world-less code path; default world is a value bound at the boundary; constructors reject a missing world.
- At-most-one prime per (world, entity); resolution principal-independent — ACL never participates in fallback (pin with a test); provenance on the prime.
- Pushdown: the world scope expressed in pgstore SQL, not a per-row Go filter.
- Selection: wiring-site binding for fixed surfaces (no world parameter at all); capability-gated `?world=`/`--world` on editor surfaces; a constructed reader handle per operation, not a ctx value.
- Automations/Lua: contextual reads through the default world; triggering face state-addressed and raw; side-effect relation tails follow relation scope.

World templates (§4.5, axis fallback + for_each) are marked tentative — do not
build until a real world family exists.
