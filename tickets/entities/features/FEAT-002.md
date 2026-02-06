---
id: FEAT-002
type: feature
title: Sticky Headers in Data Entry UI
summary: Page headers, filter bars, and jump bars stay visible while scrolling
description: |
  Sticky positioning for key UI elements in the data-entry web app:
  - Page header: sticky at top (z-index 50)
  - Filter bar: sticky below header (z-index 40), with border/shadow appearing only when stuck
  - Jump bar: sticky below header (z-index 40) with subtle shadow

  Uses IntersectionObserver to detect when filter bar becomes stuck and applies
  visual feedback (border + shadow) only in that state.
priority: medium
status: in-progress
---

## Implementation

### CSS Changes (templates.go)
- `.page-header`: `position: sticky; top: 0`
- `.filter-bar`: `position: sticky; top: 57px`
- `.filter-bar.is-stuck`: border + shadow styles
- `.jump-bar`: `position: sticky; top: 57px`

### JavaScript
- IntersectionObserver watches a sentinel div inserted before filter-bar
- Toggles `is-stuck` class when sentinel leaves viewport
