---
id: RR-4JTM7
type: review-response
title: Double role="combobox" — invalid ARIA
finding: 'CommandPaletteModal.vue:217 and :227 both carry role="combobox". ARIA 1.2 says one combobox per widget; the input owns it, the wrapping div is presentational. AT software (NVDA/VoiceOver) gets confused when nested combobox roles point at the same listbox via aria-controls. Fix: drop role="combobox", aria-controls, aria-haspopup, aria-expanded from the cmdk-modal div. Keep role="dialog" on the overlay; modal div has no ARIA role.'
severity: critical
resolution: Removed role="combobox", aria-controls, aria-haspopup, aria-expanded from the cmdk-modal div. Kept role="dialog" on the overlay, role="combobox" on the input only, and moved aria-expanded to the input where it semantically belongs. Now matches WAI-ARIA 1.2 combobox-with-listbox pattern correctly.
status: addressed
---
