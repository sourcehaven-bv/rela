---
id: FEAT-Y6BDT
type: feature
title: Human-friendly date formatting in data-entry display
summary: Render date values with abbreviated month names (e.g. "15 Jan 2024") in lists and detail views to avoid ambiguous numeric formats.
description: Date properties currently render with `toLocaleDateString()`, which produces locale-dependent numeric formats (e.g. `1/15/2024` vs `15/1/2024`) that are ambiguous between US and EU readers. Render dates with a short abbreviated month name (e.g. `15 Jan 2024`) in list cells and entity detail views so the day/month order is unambiguous while staying compact enough for table cells.
status: proposed
---
