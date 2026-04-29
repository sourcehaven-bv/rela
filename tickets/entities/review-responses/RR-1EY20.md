---
id: RR-1EY20
type: review-response
title: Trigger element gets detached by optimistic row removal
finding: 'Click path: ev.currentTarget is the .action-header-btn in a row that is v-if="hasSelection". After onClearSelection() runs, hasSelection flips to false and the button unmounts. Keyboard path: e.target may be a row removed by the optimistic filter at executeAction line 107. Either way, triggeringEl.focus() in scriptError.ts runs against a detached node and silently no-ops. Focus falls to <body>. User-visible consequence: keyboard user has to Tab from the top of the page to recover. Fix: in scriptError.ts dismiss(), guard with document.contains(triggeringEl) before focusing; fall back to a sensible target or document the limitation.'
severity: significant
resolution: Added document.contains(triggeringEl) guard in scriptError.ts dismiss() before calling .focus(). When the trigger element has been detached (e.g. by useListActions optimistic row removal), focus restore is skipped so it falls through to <body> as expected, instead of being silently no-opped. Inline comment documents the reason.
status: addressed
---

Also added regression test 'dismiss skips focus restore when the trigger has
been detached' to scriptError.test.ts, asserting focus is NOT called on a
detached node and the store still clears its current state.
