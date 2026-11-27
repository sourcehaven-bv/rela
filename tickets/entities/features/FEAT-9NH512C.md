---
id: FEAT-9NH512C
type: feature
title: Smart lists over declared filter controls
summary: Users save values for a list's declared filter controls; rela derives the indexes those filters need.
description: Saved smart lists limited to a list's declared filter controls and their allowed operators, with postgres indexes derived from those controls (RES-7SH71TW).
priority: medium
status: proposed
---

## Description

Users save filters over a list as smart lists. A smart list holds values only
for the filters the list's operator declared in `filter_controls:`, within the
operators each control allows. Because every query shape is declared in config,
rela derives the postgres indexes those filters need.

Research: RES-7SH71TW. Option B there; options A (free expressions) and C
(operator-only lists) were rejected.

## Scope

- `ops:` allowlist per filter control, with relative date bounds.
- Indexes derived from filter controls.
- Date-shaped properties in index derivation.
- Per-user saved smart lists.
- Relation controls match on target id (TKT-ZKPA1E).

## Open questions

- Where the saved state lives: a new per-user store or an extension of piles.
- Whether built-in lists (Today, Upcoming) are operator presets or product
defaults.
- Cross-type smart lists (one Today across several types). Out of scope for
v1.
