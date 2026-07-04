---
id: TKT-U62DVR
type: ticket
title: Reposition Properties auto-save indicator inline in the section heading, hidden when idle
kind: enhancement
priority: medium
effort: s
status: in-progress
---

## Description

In the entity-detail **Properties** section, the `AutoSaveIndicator` is
absolutely positioned by `SectionEditForm.vue` with `top: -28px; right: 0`,
which lifts a persistent gray "saved" checkmark up into the empty top-right
corner of the section — detached from any content and overlapping into the
section-heading row. It reads as a checkmark floating in empty space.

## Goal

Reposition the indicator **inline next to the "Properties" section heading**
(right-aligned in the heading row), and make it **hidden when idle**:

- Idle: not shown.
- Saving: shown (spinner state).
- After a successful save: keep it briefly (short delay), then **fade out**.
- Error: shown (persists until resolved).

## Scope

- **In scope:** the properties-section `SectionEditForm` usage in `EntityDetail.vue`; positioning/visibility behaviour of the indicator in that context; the fade-out-after-save transition.
- **Out of scope:** the per-row/per-card indicator instances in the `cards`/`list` sections (which pass their own `#indicator` slot); changing the underlying `useAutoSave` save logic.

## Affected files (initial)

- `frontend/src/components/forms/SectionEditForm.vue` (absolute positioning CSS + default indicator slot)
- `frontend/src/components/forms/AutoSaveIndicator.vue` (idle-hidden + fade-out states)
- `frontend/src/components/entity/EntityDetail.vue` (Properties heading row + indicator placement)

## Prior art

Placement of `AutoSaveIndicator` was previously discussed in TKT-E6094,
TKT-IHC7B, and review-responses RR-FC1D / RR-FC2A.
