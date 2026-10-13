---
id: RR-XPFG07
type: review-response
title: Hiding the button while the anchor loads shifts the header and can drop focus
finding: With the first fix the button disappeared on each uncached entity page load and came back, shifting the header and dropping keyboard focus to body if it was on the trigger.
severity: minor
resolution: The button now stays in place while the anchor loads, in RlButton's loading state (aria-busy, aria-disabled, focus kept, click and Enter/Space suppressed, spinner only after the pending delay). The trigger drops the menu's ArrowDown/ArrowUp handler while busy so the keyboard cannot open an empty menu. Test 'does not open from the keyboard while an entity page opens' uses the real RlMenu and fails without the guard. The button is removed only when the loaded page can link nothing, as the ticket requires.
status: addressed
---
