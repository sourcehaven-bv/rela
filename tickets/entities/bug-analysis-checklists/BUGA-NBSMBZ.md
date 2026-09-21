---
id: BUGA-NBSMBZ
type: bug-analysis-checklist
title: 'Analysis: Commands and actions are unreachable on entity types that declare faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — **statically, not by running a server.** Every
claim was verified against the source at HEAD: the gate (`EntityDetail.vue:199`)
is byte-for-byte as reported; `bareEntityId` is what `CommandModal` receives
(`:2448`, computed `:156`); the entity branch parses the address
(`commands.go:427-437`) while the view branch pins `defaultViewWorld()`
(`:468`); `EntityDetail.vue` never imports `runAction`; the offers filter drops
`action` (`NextActionOffers.vue:32`) although `NextActionOffer.Action` exists
(`dataentryconfig/nextaction.go:399`). A runtime repro would add no information:
the gate is a pure computed with no server input, so `servedFace → []` is
decidable by reading it. Noted here rather than claimed as a live repro.
- [x] Minimal reproduction steps documented — in the bug body; the reporter's
three steps are exact and need no narrowing.
- [x] Environment/conditions noted — rela HEAD (2026-09-18, `6f02d2d2f`),
PostgreSQL backend, `default_world: actueel`. **Not environment-specific:** the
gate is client-side and backend-independent, so every backend and every world is
affected. The one condition that matters is that the entity type declares
`faces:`.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3) — why3 is evidenced by
`git log -L 199,199`: the gate entered as `readOnly` (a world predicate,
`e0187047`) and was re-keyed to `servedFace` (a face predicate) by TKT-SLFURL
(`f9b0052f`). The commit that changed the predicate's kind is the commit whose
subject is "the face is part of the address".
- [x] Systemic cause explored (why4-5) — same failure mode as BUG-64MU2Q
(capability gap closed with prose that reads as finished). Recorded in
`prevention`.

## Fix Planning

- [x] Fix approach determined — see "Fix plan" in the bug body. Three
independent changes; (1) is the outage and stands alone.
- [x] Regression test planned — `faced-type-affordance-parity-test`
(`adds-measure`): assert affordances are **reachable** on a faced fixture, not
merely that the guard fires. A suppression-only assertion is precisely what let
this through.
- [x] Related areas checked for similar issues — audited every surface keyed on
the same "a bare address exists" assumption:
  - **TKT-2RQMV4** — Lua/SPA/CLI/MCP create cannot name a face. Same root,
already ticketed; `luaCreateEntity` (`runtime.go:1730`) is its Lua instance.
Referenced, not duplicated.
  - **BUG-64MU2Q** (done) — relation writes, same shape, already fixed.
  - **TKT-K3W8VD** (backlog) — CLI corpus commands skip non-bare faces.
  - `EntityToTable` (`runtime.go:1325-1368`) omits `face`, so no Lua consumer
(action, automation, script) can tell which face invoked it. Folded in as fix
item 3.
  - Documentation drift at `docs/content-states.md:866` — conflates create
with update; update works via the fused address. Folded into the bug body.
