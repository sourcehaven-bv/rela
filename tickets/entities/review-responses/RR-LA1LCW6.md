---
id: RR-LA1LCW6
type: review-response
title: Slash shortcut test had no positive case
finding: Sidebar.builtins.test.ts only asserted push was not called when Search is hidden.
severity: minor
resolution: Sidebar.shortcut.test.ts already asserts / pushes /search with every row shown (no hidden_builtins), so the pair is covered.
status: addressed
---
