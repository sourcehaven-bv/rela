---
id: RR-2H6YE
type: review-response
title: Tab key behavior unspecified — no focus trap means Tab leaks to background
finding: 'Plan skips a focus trap because useFocusTrap is not yet implemented. Defensible *if* Tab is handled — but the plan is silent. Without intervention, Tab from the palette input moves focus to the next tabbable element behind the modal overlay. The user then tabs around an invisible page while the palette stays open. Visually broken and a11y-broken. Simplest fix: handle Tab and Shift+Tab on the input with e.preventDefault() — palette has only one tabbable element + a listbox controlled via aria-activedescendant, so trapping Tab to the input is correct. Add: in Approach, list Tab/Shift+Tab among the keys handled on the input. Test: open palette, press Tab, assert document.activeElement is still the palette input.'
severity: significant
resolution: 'Plan updated: the input''s keydown handler calls e.preventDefault() on Tab and Shift+Tab so focus cannot escape the palette. Combined with aria-activedescendant on the input (RR-QL4SD), the palette is keyboard-trappable without a full useFocusTrap composable. Test added: open palette, press Tab and Shift+Tab, assert document.activeElement is still the palette input.'
status: addressed
---
