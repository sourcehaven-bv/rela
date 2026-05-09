---
id: RR-WQRYA
type: review-response
title: 'Naming: ''command palette'' implies actions; this iteration is entity-only'
finding: 'Throughout the plan and the in-app KeyboardShortcutsModal row, the feature is called ''command palette''. In its current scope it has no commands — only entity navigation. ''Command palette'' sets expectations the iteration does not meet. The ticket title (''Quick-search/jump command palette'') already acknowledges this tension. Two choices: (a) keep ''command palette'' in code/UI (acknowledges this is the foundation for future commands), or (b) use ''Quick jump'' / ''Quick search'' in user-visible strings (KeyboardShortcutsModal) while keeping CommandPaletteModal.vue as the file name. Defer to user.'
severity: nit
resolution: 'User chose ''Quick jump''. KeyboardShortcutsModal row label: ''Cmd/Ctrl+K — Quick jump''. Component file name stays CommandPaletteModal.vue for forward compatibility with a future commands iteration.'
status: addressed
---
