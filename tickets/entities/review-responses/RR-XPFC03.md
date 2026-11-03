---
id: RR-XPFC03
type: review-response
title: The PageView test did not exercise the unmount reset of opening
finding: PageView.test.ts checked opening after unmount only once the anchor had loaded, when opening was already false. Deleting the onBeforeUnmount reset left the test green.
severity: significant
resolution: 'Added ''clears opening when it leaves before the anchor loads'': it unmounts while the anchor fetch is pending. Verified it fails when the reset line is removed.'
status: addressed
---
