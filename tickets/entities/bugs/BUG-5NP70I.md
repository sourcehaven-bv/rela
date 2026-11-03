---
id: BUG-5NP70I
type: bug
title: Pile writes fail in a non-default world; Create pile button is off-centre
description: 'The New pile dialog fails to create a pile while the user browses a non-default world (toast: omit ?world= — address the face directly by id). Rename/add/remove fail the same way. The Create pile button label also sits right of centre because RlButton reserves an empty spinner slot for any button with a pending label.'
priority: high
effort: s
why1: usePiles passes worldParam to createPile/updatePile/addPileItems/removePileItems, which append ?world= to a POST or PATCH. attachWorld refuses ?world= on every non-read method with 422 world_read_only. Separately, RlButton reserves an empty spinner slot plus a gap before the label of any button with a pending-label and no icon, so the idle label sits right of centre.
why2: 'The piles API module threads the world through every call by symmetry with the reads, whose counts are world-scoped. The writes do not need it: the SPA already sends face-qualified addresses (entityRef gives ID@face), and the server resolves a named face literally in any world.'
why3: No test covers the SPA write against the real middleware. The Go piles tests inject the world into the request context directly (pileContext), bypassing attachWorld, so a POST in a non-default world passes there although the real stack refuses it. The frontend tests mock the api module.
why4: The world_read_only rule lives in middleware, but handler tests are built to run without it. A test helper that binds a world to a write models a request the server cannot receive.
why5: The read/write world contract (reads take ?world=, writes address faces) is not encoded at the SPA api-client layer, so each feature decides per call whether to pass the world. For the button, the no-reflow guarantee was solved by reserving permanent space, and no visual check of an idle pending-label button exists.
prevention: TestPiles_WriteThroughWorldMiddleware runs pile writes behind the real attachWorld middleware against a draft-hiding default world. api/piles.test.ts pins that reads carry the world and writes do not. The RlButton Pending story asserts in a real browser that an idle pending-label button is centred.
started: "2026-10-13"
completed: "2026-10-14"
status: done
---

## Description

Two problems in the New pile dialog (FEAT-4FLTQ9).

1. **Create fails while browsing a non-default world.** On an entity page of a faced type viewed in a world (for example the concept world, with "View Vastgesteld" shown), "Create pile" fails with the toast `omit ?world= — address the face directly by id (`ID`or`ID@face`)`. The same happens for rename, add and remove.
2. **The Create pile button looks wrong.** Its label sits right of centre: the left padding is visibly larger than the right.

## Cause

1. `usePiles` passes `worldParam` to `createPile`, `updatePile`, `addPileItems` and `removePileItems`, which append `?world=` to a POST/PATCH. `attachWorld` (`internal/dataentry/world.go`) refuses `?world=` on every non-read method with 422 `world_read_only`. The items are already face-qualified by `entityRef` (`ID@face` from `_self`), so the world is not needed on writes.
2. `RlButton` with `pending-label` and no icon always reserves an empty spinner slot plus a gap before the label (`reservesSpinner`), so an idle button is padded on the left only.

## Reproduction

Open a faced entity in a non-default world, open Add to pile → Add to new pile…,
enter a name, press Create pile.
