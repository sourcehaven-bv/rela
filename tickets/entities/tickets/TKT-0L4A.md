---
id: TKT-0L4A
type: ticket
title: Implement mobile-friendly responsive page layout and iOS viewport handling
kind: enhancement
priority: medium
effort: l
status: done
description: Add shared PageLayout/PageTitle/HelpButton components and useVisualViewportOffset composable; apply sticky-topbar/safe-area/mobile-actionbar treatment across data-entry views.
---

Squashed from earlier mobile/iOS WIP and rebased onto current develop.
Reconciled against develop's config-driven EntityDetail (which absorbed the
deleted CustomView). Embedding/SSE Go work and BUG-RJKXF tickets that were in
the same checkout are intentionally excluded from this PR.
