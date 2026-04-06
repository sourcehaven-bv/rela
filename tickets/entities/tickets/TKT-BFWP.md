---
id: TKT-BFWP
type: ticket
title: Fix palette live preview for derived CSS variables
kind: enhancement
priority: medium
effort: s
status: done
---

# Fix Palette Live Preview for Derived CSS Variables

Live preview only applied the 8 direct role colors, not the 6 derived variables
(card-bg, input-bg, hover-bg, border-color, muted-text, sidebar-text). Ported
deriveTheme() to TypeScript so the preview computes all 21 CSS variables
client-side. Also added reset button and auto-revert on navigation.
