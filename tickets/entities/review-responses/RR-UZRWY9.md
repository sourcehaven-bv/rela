---
id: RR-UZRWY9
type: review-response
title: OpenChannel is near the 500-line cap and does too much
finding: 'OpenChannel.vue is 491 lines: session state, the composer flow, five message operations, lifecycle, the ephemeral expiry clock, the jump loop and the ref picker. The next feature pushes it over max-lines. Extract the expiry clock (lines 110-134) into a composable such as useExpiryClock(signals), and the picker promise plumbing (lines 385-404) into useRefPicker(). Both can then be tested without mounting.'
severity: minor
resolution: The expiry clock, the ref picker and the pending-key set moved into useExpiryClock, useRefPicker and usePendingKeys with their own tests, bringing OpenChannel to 463 lines.
status: addressed
---
