---
id: RR-FC2A
type: review-response
title: 'Round 2 NEW-2: Step 5 reopens the indicator-placement decision'
finding: |
  Step 5's templated example renders `<AutoSaveIndicator>` in the card header AND wires SectionEditForm with `<template #indicator>` empty. Then says "Pick whichever is cleaner during implementation." That re-opens the AC 6 commitment to slot-based placement. Implementation must not be left to choose between two semantically different approaches.
severity: significant
status: addressed
resolution: |
  PLAN Step 5 committed: indicator placement is HOST-CONTROLLED VIA THE SLOT, period. The cards example shows:
    - Card header chrome (entity-type / title / id / edit button) WITHOUT an inline AutoSaveIndicator.
    - SectionEditForm uses `<template #indicator>` to teleport the indicator into the card header via a Vue `<Teleport>` to a marker `<span class="card-indicator-slot">` placed in the header.

  No more `rowStatus`/`rowError` accessor escape hatch. The slot mechanism handles all the state flow; the host's only job is "where do I render this".

  Same pattern for list rows: `<span class="list-indicator-slot">` placed inline-right of the title link.
---
