---
id: RR-KQKRG
type: review-response
title: Magic 150ms debounce should be a named constant
finding: 'CommandPaletteModal.vue:82 inlines 150 as a magic number. Fix: hoist to `const DEBOUNCE_MS = 150` at the top with a one-line comment (''perceptually-instant on a fast connection; tune up if API is slow'').'
severity: nit
resolution: Hoisted to const DEBOUNCE_MS = 150 with a one-line comment explaining tuning. Same treatment for MIN_QUERY_LEN and MAX_RESULTS (added together).
status: addressed
---
