---
id: BUGA-NLLJDM
type: bug-analysis-checklist
title: 'Analysis: Selection relation not saved when creating an entity in data-entry (edit works)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (live rela-server + real SPA driven via Puppeteer; captured the create POST body omitting the incoming edge)
- [x] Minimal reproduction steps documented (create form with a `direction: incoming` picker → select peer → Create → edge dropped; edit works)
- [x] Environment/conditions noted (affects any `direction: incoming` non-cards relation picker on the create form; independent of project)

## Root Cause

- [x] Immediate cause identified (why1): incoming picker never emits the selection on create
- [x] Contributing factors found (why2-3): `emitIncomingDiff` early-returns while `incomingLoaded` is false; `loadIncomingValue` short-circuits without an entityId; TKT-GFQK load-failure guard collapses "nothing to load" into "load failed"
- [x] Systemic cause explored (why4-5): create-mode incoming pickers never tested; asymmetric emit channels for the two directions

## Fix Planning

- [x] Fix approach determined: in `RelationPicker.loadIncomingValue`, treat "no entityId" (create) as an empty loaded baseline (`incomingLoaded = true`) so selections emit as pure additions
- [x] Regression test planned: unit (RelationPicker incoming-on-create emits) + e2e (create feature form, incoming picker persists) — see `incoming-picker-create-persist-test`; both verified to fail before the fix
- [x] Related areas checked for similar issues: RelationCards (the other incoming widget) never renders on create (requires entityId), so no parallel create-mode drop
