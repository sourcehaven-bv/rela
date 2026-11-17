---
id: RR-A12KTC1
type: review-response
title: Keyboard readout not reliably announced
finding: The tooltip was a role=status region created by v-if, which screen readers often miss.
severity: minor
resolution: A permanently mounted visually hidden aria-live region carries the readout text; the visual tooltip is aria-hidden. The table view stays the full fallback.
status: addressed
---
